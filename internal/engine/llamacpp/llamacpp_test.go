package llamacpp

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/pcfpcfpcf/steth/internal/engine/enginetest"
	"github.com/pcfpcfpcf/steth/internal/engine/local"
)

const child = "?autoload=false&model=qwen3.5-0.8b"

// newSource reads `url` with `procs` as the machine's processes; nil is a
// machine where none can be seen.
func newSource(url string, client *http.Client, procs enginetest.Processes) *Source {
	s := New(url, client)
	s.local.Processes = procs
	return s
}

func TestReadMatchesSchema(t *testing.T) {
	cases := map[string]map[string]string{
		"single": {
			"/props":   "testdata/live.props.json",
			"/slots":   "testdata/live.slots.json",
			"/metrics": "testdata/live.metrics.1.txt",
		},
		// No /slots: reading it would wake the server.
		"sleeping": {
			"/props": "testdata/sleeping.props.json",
		},
		"router": {
			"/props":         "testdata/router.props.json",
			"/v1/models":     "testdata/router.models.json",
			"/props" + child: "testdata/router.child.props.json",
			"/slots" + child: "testdata/router.child.slots.json",
		},
	}
	for name, fixtures := range cases {
		t.Run(name, func(t *testing.T) {
			server := enginetest.Serve(t, fixtures)
			reading, err := newSource(server.URL, server.Client(), nil).Read(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			enginetest.Match(t, reading, "testdata/"+name+".want.json")
		})
	}
}

// Speeds come from the counters' change between two reads, so the first
// read has none.
func TestSpeedsFromTwoReads(t *testing.T) {
	server := enginetest.Serve(t, map[string]string{
		"/props":   "testdata/live.props.json",
		"/slots":   "testdata/live.slots.json",
		"/metrics": "testdata/live.metrics.1.txt",
	})
	source := newSource(server.URL, server.Client(), nil)
	first, err := source.Read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if m := first.Models[0]; m.DecodeTPS != nil || m.PrefillTPS != nil {
		t.Error("first read has a speed; expected none without an earlier read")
	}
	server.Answer("/metrics", "testdata/live.metrics.2.txt")
	second, err := source.Read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	enginetest.Match(t, second, "testdata/speeds.want.json")
}

// A server whose process can be seen: its memory, its command line, the
// whole of a KV cache split between four slots, and no /slots read, since
// that would keep it from sleeping.
func TestVisibleProcess(t *testing.T) {
	server := enginetest.Serve(t, map[string]string{
		"/props":   "testdata/live.props.json",
		"/metrics": "testdata/live.metrics.1.txt",
	})
	port := local.Port(server.URL)
	procs := enginetest.Processes{port: {
		PID:  4242,
		Args: "llama serve -m /models/Qwen3.5-0.8B-Q8_0.gguf -np 4 --sleep-idle-seconds 300 --metrics",
		Held: map[string]uint64{"vram": 1 << 30, "dram": 2 << 30},
	}}
	reading, err := newSource(server.URL, server.Client(), procs).Read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	enginetest.Match(t, reading, "testdata/process.want.json")
}

// While a request generates, its speed comes from the slot's decoded tokens:
// 124, then 152, 1.06 s apart, recorded from a running server.
func TestLiveDecodeFromSlots(t *testing.T) {
	server := enginetest.Serve(t, map[string]string{
		"/props":   "testdata/live.props.json",
		"/slots":   "testdata/live.slots.gen.1.json",
		"/metrics": "testdata/live.metrics.1.txt",
	})
	source := newSource(server.URL, server.Client(), nil)
	clock := time.Date(2026, 10, 26, 19, 0, 0, 0, time.UTC)
	source.now = func() time.Time { return clock }
	if _, err := source.Read(context.Background()); err != nil {
		t.Fatal(err)
	}
	server.Answer("/slots", "testdata/live.slots.gen.2.json")
	clock = clock.Add(1060819149 * time.Nanosecond)
	reading, err := source.Read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	m := reading.Models[0]
	if m.State != "serving" || m.DecodeTPS == nil || *m.DecodeTPS < 26.3 || *m.DecodeTPS > 26.5 {
		t.Errorf("state %s, decode %v; expected serving at 28 tokens / 1.06 s = 26.4 tok/s", m.State, m.DecodeTPS)
	}
}

func TestLaunch(t *testing.T) {
	for args, want := range map[string][2]bool{ // unified KV, sleeps
		"llama serve -m m.gguf":                         {true, false},
		"llama serve -m m.gguf -np 2":                   {false, false},
		"llama serve -m m.gguf -np 2 -kvu":              {true, false},
		"llama serve -m m.gguf -no-kvu":                 {false, false},
		"llama serve -m m.gguf --sleep-idle-seconds 60": {true, true},
		"llama serve -m m.gguf --sleep-idle-seconds -1": {true, false},
	} {
		l := launch(strings.Fields(args))
		if got := [2]bool{l.unifiedKV(), l.sleeps()}; got != want {
			t.Errorf("%q: unified, sleeps = %v, expected %v", args, got, want)
		}
	}
	if port := launch(strings.Fields("llama serve --port 41001")).port(); port != 41001 {
		t.Errorf("port %d, expected 41001", port)
	}
}

func TestReadFailsOnBadStatus(t *testing.T) {
	server := enginetest.Serve(t, map[string]string{})
	if _, err := newSource(server.URL, server.Client(), nil).Read(context.Background()); err == nil {
		t.Fatal("expected an error for a 404")
	}
}

func TestRuns(t *testing.T) {
	for cmdline, want := range map[string]bool{
		"/opt/llama/bin/llama serve -m model.gguf":    true,
		"llama server -hf ggml-org/Qwen3.5-0.8B-GGUF": true,
		"llama cli -m model.gguf":                     false,
		"/usr/bin/llama-server -m model.gguf":         true,
		"llama":                                       false,
	} {
		if got := Runs(strings.Fields(cmdline)); got != want {
			t.Errorf("%q: %v, expected %v", cmdline, got, want)
		}
	}
}

func TestProbe(t *testing.T) {
	for file, want := range map[string]bool{
		"testdata/live.props.json":        true,
		"testdata/router.props.json":      true,
		"../ollama/testdata/version.json": false,
	} {
		server := enginetest.Serve(t, map[string]string{"/props": file})
		if got := Probe(context.Background(), server.Client(), server.URL); got != want {
			t.Errorf("%s: %v, expected %v", file, got, want)
		}
	}
}
