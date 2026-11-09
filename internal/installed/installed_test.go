package installed_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/pcfpcfpcf/steth/internal/engine/geode"
	"github.com/pcfpcfpcf/steth/internal/engine/llamacpp"
	"github.com/pcfpcfpcf/steth/internal/engine/ollama"
	"github.com/pcfpcfpcf/steth/internal/engine/vllm"
	"github.com/pcfpcfpcf/steth/internal/installed"
)

var engines = []installed.Engine{
	{Name: "geode", Binaries: geode.Binaries},
	{Name: "ollama", Binaries: ollama.Binaries},
	{Name: "llamacpp", Binaries: llamacpp.Binaries},
	{Name: "vllm", Binaries: vllm.Binaries},
}

// program writes an executable shell script that prints says, in a new
// directory unless dir is given, and returns its path.
func program(t *testing.T, dir, name, says string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the fake programs are shell scripts")
	}
	if dir == "" {
		dir = t.TempDir()
	}
	path := filepath.Join(dir, name)
	script := "#!/bin/sh\nprintf '%s\\n' '" + says + "'\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func find(t *testing.T, path string, named ...installed.Named) map[string]installed.Found {
	t.Helper()
	found, err := installed.Find(context.Background(), engines, path, named)
	if err != nil {
		t.Fatal(err)
	}
	byEngine := map[string]installed.Found{}
	for _, f := range found {
		byEngine[f.Engine] = f
	}
	return byEngine
}

func TestFindsEachEngineOnPath(t *testing.T) {
	first, second := t.TempDir(), t.TempDir()
	geodeAt := program(t, first, "geode", "unused")
	ollamaAt := program(t, first, "ollama", "ollama version is 0.21.2")
	// A program called llama that is not llama.cpp's does not count; the
	// llama-server after it does.
	program(t, first, "llama", "Usage: llama [a different tool]")
	serverAt := program(t, second, "llama-server", "version: 10217 (ddd4ec142)\nbuilt with Clang 22.1.8")
	// A file called vllm that is not executable is not a program.
	if err := os.WriteFile(filepath.Join(second, "vllm"), []byte("#!/bin/sh\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// A relative directory on PATH is never looked in.
	path := strings.Join([]string{"relative", first, second, first}, string(os.PathListSeparator))

	got := find(t, path)
	for _, want := range []installed.Found{
		{Engine: "geode", Path: geodeAt},
		{Engine: "ollama", Path: ollamaAt, Version: "0.21.2"},
		{Engine: "llamacpp", Path: serverAt, Version: "b10217"},
	} {
		f := got[want.Engine]
		if f.Path != want.Path || f.Version != want.Version || f.Named {
			t.Errorf("%s: got %+v; expected %+v", want.Engine, f, want)
		}
	}
	if f, ok := got["vllm"]; ok {
		t.Errorf("vllm is not installed; found %+v", f)
	}
	if got["llamacpp"].Binary.Name != "llama-server" {
		t.Errorf("llama.cpp's binary %+v; expected llama-server's", got["llamacpp"].Binary)
	}
	if len(got) != 3 {
		t.Errorf("found %d engines; expected 3: %+v", len(got), got)
	}
}

// llama.cpp's own `llama` says its build, and is preferred.
func TestLlamaCountsWhenItSaysItsBuild(t *testing.T) {
	dir := t.TempDir()
	llama := program(t, dir, "llama", "b10217-ddd4ec142")
	program(t, dir, "llama-server", "version: 10100 (abc1234)")
	f := find(t, dir)["llamacpp"]
	if f.Path != llama || f.Version != "b10217" || f.Binary.Name != "llama" {
		t.Errorf("got %+v; expected llama, build b10217", f)
	}
}

func TestNoProgramIsNoEngine(t *testing.T) {
	if got := find(t, t.TempDir()); len(got) != 0 {
		t.Errorf("an empty PATH: found %+v", got)
	}
}

// A named program is taken as its engine's whatever its name, and wins
// over the one on PATH.
func TestNamedProgramIsTrusted(t *testing.T) {
	onPath := t.TempDir()
	program(t, onPath, "geode", "")
	mine := program(t, "", "my-geode-build", "")
	f := find(t, onPath, installed.Named{Engine: "geode", Path: mine})["geode"]
	if f.Path != mine || !f.Named || f.Binary.Name != "geode" || f.Binary.Serve == nil {
		t.Errorf("got %+v; expected the named program, served as geode", f)
	}
	for _, c := range []struct {
		named installed.Named
		says  string
	}{
		{installed.Named{Engine: "geode", Path: "bin/geode"}, "give the program's absolute path"},
		{installed.Named{Engine: "geode", Path: filepath.Join(onPath, "gone")}, "give the path of an executable file"},
		{installed.Named{Engine: "tgi", Path: mine}, "no engine is named tgi; name one of geode, ollama, llamacpp, vllm"},
	} {
		_, err := installed.Find(context.Background(), engines, onPath, []installed.Named{c.named})
		if err == nil || !strings.Contains(err.Error(), c.says) {
			t.Errorf("%+v: got %v; expected an error saying %q", c.named, err, c.says)
		}
	}
}

func TestRunsByProgramName(t *testing.T) {
	llama := engines[2]
	for program, want := range map[string]bool{
		"/usr/bin/llama-server": true, "llama": true, `C:\llama\llama-server.exe`: runtime.GOOS == "windows",
		"/usr/bin/llamafile": false, "geode": false,
	} {
		if got := llama.Runs(program); got != want {
			t.Errorf("Runs(%q) = %v; expected %v", program, got, want)
		}
	}
}
