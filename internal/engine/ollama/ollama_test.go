package ollama

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/pcfpcfpcf/steth/internal/engine/enginetest"
)

// When the fixtures were recorded, so their expiry times are an hour ahead.
var recorded = time.Date(2026, 10, 26, 15, 11, 0, 0, time.UTC)

// Recorded from Ollama 0.21.2: one model wholly on the GPU; one split
// between GPU and host memory beside one on the CPU only; none loaded.
func TestReadMatchesSchema(t *testing.T) {
	for _, name := range []string{"vram", "two", "none"} {
		t.Run(name, func(t *testing.T) {
			server := enginetest.Serve(t, map[string]string{"/api/ps": "testdata/" + name + ".json"})
			source := New(server.URL, server.Client())
			source.now = func() time.Time { return recorded }
			reading, err := source.Read(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			enginetest.Match(t, reading, "testdata/"+name+".want.json")
		})
	}
}

func TestReadFailsOnBadStatus(t *testing.T) {
	server := enginetest.Serve(t, map[string]string{})
	if _, err := New(server.URL, server.Client()).Read(context.Background()); err == nil {
		t.Fatal("expected an error for a 404")
	}
}

func TestKeptForGoodNeverUnloads(t *testing.T) {
	var w wireModel
	w.ExpiresAt = time.Date(2318, 2, 7, 0, 0, 0, 0, time.UTC)
	if m := toModel(w, recorded); m.UnloadsAt != nil {
		t.Errorf("unloads at %v; expected never", m.UnloadsAt)
	}
	w.ExpiresAt = recorded.Add(5 * time.Minute)
	if m := toModel(w, recorded); m.UnloadsAt == nil {
		t.Error("expected an unload time five minutes ahead")
	}
}

func TestRuns(t *testing.T) {
	for cmdline, want := range map[string]bool{
		"/usr/local/bin/ollama serve":                                    true,
		"/usr/local/bin/ollama runner --model /models/blob --port 40411": false,
		"ollama": false,
	} {
		if got := Runs(strings.Fields(cmdline)); got != want {
			t.Errorf("%q: %v, expected %v", cmdline, got, want)
		}
	}
}

func TestProbe(t *testing.T) {
	ollama := enginetest.Serve(t, map[string]string{"/api/version": "testdata/version.json"})
	if !Probe(context.Background(), ollama.Client(), ollama.URL) {
		t.Error("Ollama not recognised")
	}
	other := enginetest.Serve(t, map[string]string{"/api/version": "../llamacpp/testdata/single.props.json"})
	if Probe(context.Background(), other.Client(), other.URL) {
		t.Error("another server's JSON taken for Ollama")
	}
}
