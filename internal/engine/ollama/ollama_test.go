package ollama

import (
	"context"
	"testing"

	"github.com/pcfpcfpcf/steth/internal/engine/enginetest"
)

// Recorded from Ollama 0.21.2: one model wholly on the GPU; one split
// between GPU and host memory beside one on the CPU only; none loaded.
func TestReadMatchesSchema(t *testing.T) {
	for _, name := range []string{"vram", "two", "none"} {
		t.Run(name, func(t *testing.T) {
			server := enginetest.Serve(t, map[string]string{"/api/ps": "testdata/" + name + ".json"})
			reading, err := New(server.URL, server.Client()).Read(context.Background())
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
