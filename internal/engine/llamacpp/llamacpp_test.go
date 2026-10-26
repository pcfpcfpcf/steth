package llamacpp

import (
	"context"
	"strings"
	"testing"

	"github.com/pcfpcfpcf/steth/internal/engine/enginetest"
)

const child = "?autoload=false&model=qwen3.5-0.8b"

func TestReadMatchesSchema(t *testing.T) {
	cases := map[string]map[string]string{
		"single": {
			"/props": "testdata/single.props.json",
			"/slots": "testdata/single.slots.json",
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
			reading, err := New(server.URL, server.Client()).Read(context.Background())
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
		"/props":   "testdata/metrics.props.json",
		"/slots":   "testdata/single.slots.json",
		"/metrics": "testdata/metrics.1.txt",
	})
	source := New(server.URL, server.Client())
	first, err := source.Read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if m := first.Models[0]; m.DecodeTPS != nil || m.PrefillTPS != nil {
		t.Error("first read has a speed; expected none without an earlier read")
	}
	server.Answer("/metrics", "testdata/metrics.2.txt")
	second, err := source.Read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	enginetest.Match(t, second, "testdata/metrics.want.json")
}

func TestReadFailsOnBadStatus(t *testing.T) {
	server := enginetest.Serve(t, map[string]string{})
	if _, err := New(server.URL, server.Client()).Read(context.Background()); err == nil {
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
		"testdata/single.props.json":      true,
		"testdata/router.props.json":      true,
		"../ollama/testdata/version.json": false,
	} {
		server := enginetest.Serve(t, map[string]string{"/props": file})
		if got := Probe(context.Background(), server.Client(), server.URL); got != want {
			t.Errorf("%s: %v, expected %v", file, got, want)
		}
	}
}
