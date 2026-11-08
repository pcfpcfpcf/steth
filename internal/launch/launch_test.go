package launch

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

const (
	helperEnv = "STETH_LAUNCH_HELPER"
	// helperLife bounds a helper that a failed test leaves running.
	helperLife = time.Minute
	waitLimit  = 10 * time.Second
	pollEvery  = 10 * time.Millisecond
	shortGrace = 100 * time.Millisecond
)

// TestHelperProcess is the child the other tests start: this test binary
// run again with helperEnv set. Its arguments after "--" say what to do:
//
//	lines COUNT LENGTH  print COUNT numbered lines padded to LENGTH, then sleep
//	echo ARG...         print each ARG on a line, then sleep
//	exit CODE           print "leaving" and exit with CODE
//	deaf                ignore interrupts, print "ready", then sleep
//	spawn               start a sleeping helper, print "grandchild PID", then sleep
func TestHelperProcess(t *testing.T) {
	if os.Getenv(helperEnv) != "1" {
		return
	}
	args := os.Args[slices.Index(os.Args, "--")+1:]
	switch args[0] {
	case "lines":
		count, _ := strconv.Atoi(args[1])
		length, _ := strconv.Atoi(args[2])
		for i := range count {
			line := fmt.Sprintf("line %04d ", i)
			fmt.Println(line + strings.Repeat("x", max(0, length-len(line))))
		}
	case "echo":
		for _, arg := range args[1:] {
			fmt.Println(arg)
		}
	case "exit":
		code, _ := strconv.Atoi(args[1])
		fmt.Println("leaving")
		os.Exit(code)
	case "deaf":
		signal.Ignore(os.Interrupt)
		fmt.Println("ready")
	case "spawn":
		grandchild := exec.Command(os.Args[0], "-test.run=^TestHelperProcess$", "--", "lines", "0", "0")
		if err := grandchild.Start(); err != nil {
			fmt.Println(err)
			os.Exit(2)
		}
		fmt.Println("grandchild", grandchild.Process.Pid)
	default:
		fmt.Println("unknown helper mode", args[0])
		os.Exit(2)
	}
	time.Sleep(helperLife)
	os.Exit(0)
}

func helper(t *testing.T, name string, args ...string) Profile {
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	command := append([]string{self, "-test.run=^TestHelperProcess$", "--"}, args...)
	return Profile{Name: name, Command: command, Env: map[string]string{helperEnv: "1"}}
}

func launcher(t *testing.T, profiles ...Profile) *Launcher {
	l := New(profiles)
	l.grace = shortGrace
	t.Cleanup(l.StopAll)
	return l
}

func status(t *testing.T, l *Launcher, name string) Status {
	t.Helper()
	for _, s := range l.Status() {
		if s.Name == name {
			return s
		}
	}
	t.Fatalf("no status for %q", name)
	return Status{}
}

func waitFor(t *testing.T, what string, done func() bool) {
	t.Helper()
	deadline := time.Now().Add(waitLimit)
	for !done() {
		if time.Now().After(deadline) {
			t.Fatalf("gave up after %s waiting for %s", waitLimit, what)
		}
		time.Sleep(pollEvery)
	}
}

func waitForLine(t *testing.T, l *Launcher, name, line string) {
	t.Helper()
	waitFor(t, fmt.Sprintf("%q in %s's log", line, name), func() bool {
		return slices.Contains(status(t, l, name).Log, line)
	})
}

func start(t *testing.T, l *Launcher, name, model string) {
	t.Helper()
	if err := l.Start(name, model); err != nil {
		t.Fatal(err)
	}
}

func writeConfig(t *testing.T, text string) string {
	path := filepath.Join(t.TempDir(), "steth.json")
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadReadsProfiles(t *testing.T) {
	path := writeConfig(t, `{"launch": [
		{"name": "geode", "command": ["geode", "serve", "{model}", "--port", "8080"],
		 "models": ["a.gguf", "b.gguf"], "dir": "/srv/geode", "env": {"HOME": "/home/me"}},
		{"name": "llama router", "command": ["llama", "serve", "--port", "9931"]}
	]}`)
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	want := []Profile{
		{Name: "geode", Command: []string{"geode", "serve", "{model}", "--port", "8080"},
			Models: []string{"a.gguf", "b.gguf"}, Dir: "/srv/geode", Env: map[string]string{"HOME": "/home/me"}},
		{Name: "llama router", Command: []string{"llama", "serve", "--port", "9931"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v\nexpected %+v", got, want)
	}
}

func TestLoadSaysWhatWouldWork(t *testing.T) {
	for _, c := range []struct{ file, says string }{
		{`{"launch": [{"name": "a", "command": ["x"], "cmd": ["x"]}]}`, `unknown field "cmd"; the file holds`},
		{`{"profiles": []}`, `unknown field "profiles"`},
		{`{"launch": [{"command": ["x"]}]}`, "profile 1 has no name"},
		{`{"launch": [{"name": "a"}]}`, `profile "a" has an empty command`},
		{`{"launch": [{"name": "a", "command": [""]}]}`, `profile "a" has an empty command`},
		{`{"launch": [{"name": "a", "command": ["x"]}, {"name": "a", "command": ["y"]}]}`, `two profiles are named "a"`},
		{`{"launch": [{"name": "a", "command": ["x", "--model={model}"]}]}`, `has {model} in its command but no models`},
		{`{"launch": [{"name": "a", "command": ["x"], "models": ["m"]}]}`, `lists models but its command has no {model}`},
		{`{"launch": []} {}`, "text after the first JSON object"},
		{`{"launch": [`, "unexpected EOF"},
	} {
		_, err := Load(writeConfig(t, c.file))
		if err == nil || !strings.Contains(err.Error(), c.says) {
			t.Errorf("%s: got %v; expected an error saying %q", c.file, err, c.says)
		}
	}
	if _, err := Load(filepath.Join(t.TempDir(), "missing.json")); err == nil {
		t.Error("expected an error for a file not there")
	}
}

func TestStatusShowsRunningChildAndItsLog(t *testing.T) {
	echo := helper(t, "echo", "echo", "model={model}", "second line")
	echo.Models = []string{"m1", "m2"}
	idle := helper(t, "idle", "lines", "0", "0")
	l := launcher(t, echo, idle)
	before := time.Now()
	start(t, l, "echo", "m2")
	waitForLine(t, l, "echo", "second line")

	statuses := l.Status()
	if len(statuses) != 2 || statuses[0].Name != "echo" || statuses[1].Name != "idle" {
		t.Fatalf("expected echo then idle, in config order; got %+v", statuses)
	}
	s := statuses[0]
	if !s.Running || s.Model != "m2" || s.PID == 0 || s.Exit != "" || s.Started == nil || s.Started.Before(before) {
		t.Errorf("expected echo running m2 with a pid and start time; got %+v", s)
	}
	if !slices.Equal(s.Log, []string{"model=m2", "second line"}) {
		t.Errorf("expected {model} replaced and both lines in order; got %q", s.Log)
	}
	if !slices.Equal(s.Models, echo.Models) {
		t.Errorf("got models %q", s.Models)
	}
	never := statuses[1]
	if never.Running || never.PID != 0 || never.Started != nil || never.Log == nil || never.Models == nil {
		t.Errorf("expected idle never started, with empty (not null) log and models; got %+v", never)
	}
}

func TestLogKeepsLastLinesEachCut(t *testing.T) {
	l := launcher(t, helper(t, "many", "lines", "130", "0"), helper(t, "long", "lines", "1", "5000"))
	start(t, l, "many", "")
	start(t, l, "long", "")
	waitForLine(t, l, "many", "line 0129 ")
	log := status(t, l, "many").Log
	if len(log) != maxLogLines || log[0] != "line 0010 " || log[len(log)-1] != "line 0129 " {
		t.Errorf("expected lines 10 to 129, %d of them; got %d from %q to %q", maxLogLines, len(log), log[0], log[len(log)-1])
	}
	waitFor(t, "the long line", func() bool { return len(status(t, l, "long").Log) == 1 })
	line := status(t, l, "long").Log[0]
	if utf8.RuneCountInString(line) != maxLineChars || !strings.HasPrefix(line, "line 0000 xxx") || !strings.HasSuffix(line, cutMark) {
		t.Errorf("expected the 5000-character line cut to %d ending in %q; got %d characters ending in %q",
			maxLineChars, cutMark, utf8.RuneCountInString(line), line[len(line)-10:])
	}
}

func TestLineLogBoundsAndJoinsWrites(t *testing.T) {
	var g lineLog
	g.write([]byte("first half "))
	g.write([]byte("second half\r\nnext"))
	g.write([]byte(strings.Repeat("é", 3*maxLineChars)))
	g.write([]byte("\n\nno newline"))
	g.finish()
	got := g.lines()
	if len(got) != 4 || got[0] != "first half second half" || got[2] != "" || got[3] != "no newline" {
		t.Fatalf("got %q", got)
	}
	if utf8.RuneCountInString(got[1]) != maxLineChars || !strings.HasPrefix(got[1], "nextéé") || !utf8.ValidString(got[1]) {
		t.Errorf("expected a valid line cut to %d characters; got %d", maxLineChars, utf8.RuneCountInString(got[1]))
	}
	if len(g.partial) != 0 || cap(g.partial) > maxLineBytes*2 {
		t.Errorf("expected the partial line bounded and empty; cap %d", cap(g.partial))
	}
}

func TestStopEndsChildAndKeepsLogUntilNextStart(t *testing.T) {
	echo := helper(t, "echo", "echo", "{model}")
	echo.Models = []string{"m1", "m2"}
	l := launcher(t, echo)
	start(t, l, "echo", "m1")
	waitForLine(t, l, "echo", "m1")
	pid := status(t, l, "echo").PID
	if err := l.Stop("echo"); err != nil {
		t.Fatal(err)
	}
	s := status(t, l, "echo")
	if s.Running || s.Exit != stoppedExit || s.PID != 0 || !slices.Equal(s.Log, []string{"m1"}) || s.Model != "m1" {
		t.Errorf("expected echo stopped with its log kept; got %+v", s)
	}
	if err := l.Stop("echo"); err == nil || !strings.Contains(err.Error(), "not running") {
		t.Errorf("expected Stop to refuse a stopped profile; got %v", err)
	}

	start(t, l, "echo", "m2")
	waitForLine(t, l, "echo", "m2")
	s = status(t, l, "echo")
	if !s.Running || s.Exit != "" || slices.Contains(s.Log, "m1") || s.PID == pid {
		t.Errorf("expected a fresh run with only its own log; got %+v", s)
	}
}

func TestExitIsRecorded(t *testing.T) {
	l := launcher(t, helper(t, "quitter", "exit", "3"))
	start(t, l, "quitter", "")
	waitFor(t, "quitter to exit", func() bool { return !status(t, l, "quitter").Running })
	s := status(t, l, "quitter")
	if s.Exit != "exit status 3" || !slices.Equal(s.Log, []string{"leaving"}) {
		t.Errorf("expected exit status 3 and its log; got %+v", s)
	}
}

func TestStopKillsChildThatIgnoresInterrupt(t *testing.T) {
	l := launcher(t, helper(t, "deaf", "deaf"))
	start(t, l, "deaf", "")
	waitForLine(t, l, "deaf", "ready")
	if err := l.Stop("deaf"); err != nil {
		t.Fatal(err)
	}
	if s := status(t, l, "deaf"); s.Running || s.Exit != stoppedExit {
		t.Errorf("expected deaf killed; got %+v", s)
	}
}

func TestStartRefuses(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	taken := strconv.Itoa(listener.Addr().(*net.TCPAddr).Port)
	free, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	freePort := strconv.Itoa(free.Addr().(*net.TCPAddr).Port)
	free.Close()

	withModel := helper(t, "with model", "echo", "{model}")
	withModel.Models = []string{"m1"}
	l := launcher(t,
		helper(t, "running", "lines", "0", "0"),
		withModel,
		helper(t, "taken", "echo", "--port", taken),
		helper(t, "taken=", "echo", "--port="+taken),
		helper(t, "free", "echo", "--port="+freePort),
		helper(t, "not a port", "echo", "--port", "eighty"),
	)
	start(t, l, "running", "")
	for _, c := range []struct{ name, model, says string }{
		{"nobody", "", `no profile named "nobody"; the profiles are "running", "with model"`},
		{"running", "", `"running" is already running as pid`},
		{"running", "m1", `"running" takes no model`},
		{"with model", "m9", `"with model" has no model "m9"; choose one of "m1"`},
		{"with model", "", `"with model" has no model ""`},
		{"taken", "", "port " + taken + " already answers on 127.0.0.1"},
		{"taken=", "", "port " + taken + " already answers on 127.0.0.1"},
		{"not a port", "", `--port "eighty" is not a port`},
	} {
		err := l.Start(c.name, c.model)
		if err == nil || !strings.Contains(err.Error(), c.says) {
			t.Errorf("Start(%q, %q): got %v; expected an error saying %q", c.name, c.model, err, c.says)
		}
	}
	for _, name := range []string{"with model", "taken", "taken=", "not a port"} {
		if s := status(t, l, name); s.Started != nil {
			t.Errorf("expected %q never started; got %+v", name, s)
		}
	}
	start(t, l, "free", "")
	if err := l.Stop("nobody"); err == nil || !strings.Contains(err.Error(), `no profile named "nobody"`) {
		t.Errorf("expected Stop to refuse an unknown profile; got %v", err)
	}
	if err := New(nil).Start("x", ""); err == nil || !strings.Contains(err.Error(), "the config file has none") {
		t.Errorf("expected a launcher without profiles to say so; got %v", err)
	}
}

func TestStopAllStopsEveryChild(t *testing.T) {
	l := launcher(t, helper(t, "a", "echo", "a up"), helper(t, "b", "deaf"), helper(t, "c", "echo", "c up"))
	start(t, l, "a", "")
	start(t, l, "b", "")
	waitForLine(t, l, "a", "a up")
	waitForLine(t, l, "b", "ready")
	l.StopAll()
	for _, s := range l.Status() {
		if s.Name == "c" {
			if s.Started != nil {
				t.Errorf("expected c never started; got %+v", s)
			}
			continue
		}
		if s.Running || s.Exit != stoppedExit {
			t.Errorf("expected %s stopped; got %+v", s.Name, s)
		}
	}
}

func TestStopTakesChildrenWithIt(t *testing.T) {
	l := launcher(t, helper(t, "parent", "spawn"))
	start(t, l, "parent", "")
	var grandchild int
	waitFor(t, "the grandchild's pid", func() bool {
		for _, line := range status(t, l, "parent").Log {
			if _, err := fmt.Sscanf(line, "grandchild %d", &grandchild); err == nil {
				return true
			}
		}
		return false
	})
	gone := watchProcess(t, grandchild)
	if gone() {
		t.Fatalf("grandchild %d gone before Stop", grandchild)
	}
	if err := l.Stop("parent"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, fmt.Sprintf("grandchild %d to end", grandchild), gone)
}
