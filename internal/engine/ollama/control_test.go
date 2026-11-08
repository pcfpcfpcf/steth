package ollama

import (
	"context"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/pcfpcfpcf/steth/internal/engine/control"
	"github.com/pcfpcfpcf/steth/internal/engine/enginetest"
)

var _ control.Controls = (*Source)(nil)

// engine answers each path with its file, a GET with 200 and a POST with
// `posted`, and keeps every POST it was sent.
func engine(t *testing.T, answers map[string]string, posted int) (*Source, func() []string) {
	server := enginetest.Serve(t, answers)
	server.AnswerPosts(posted)
	return New(server.URL, server.Client()), server.Posted
}

// Recorded from Ollama 0.21.2 with nine models on disk.
func TestAvailableIsEveryModelOnDisk(t *testing.T) {
	source, _ := engine(t, map[string]string{"/api/tags": "testdata/tags.json"}, http.StatusOK)
	got, err := source.Available(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"qwen3:8b", "gigachat3.1:latest", "nomic-embed-text:latest", "qwen2.5:1.5b",
		"llama3.2:1b", "llama3.2:3b", "qwen2.5:7b", "gemma3:1b", "gemma4:e4b"}
	if !slices.Equal(got, want) {
		t.Errorf("available %q, expected %q", got, want)
	}
}

// load.json is Ollama 0.21.2 answering a load of gemma3:1b.
func TestLoadAndUnloadAskForNoPrompt(t *testing.T) {
	ctx := context.Background()
	source, sent := engine(t, map[string]string{"/api/generate": "testdata/load.json"}, http.StatusOK)
	if err := source.Load(ctx, "llama3.2:1b"); err != nil {
		t.Fatal(err)
	}
	if err := source.Unload(ctx, "llama3.2:1b"); err != nil {
		t.Fatal(err)
	}
	want := []string{
		`/api/generate {"model":"llama3.2:1b"}`,
		`/api/generate {"keep_alive":0,"model":"llama3.2:1b"}`,
	}
	if got := sent(); !slices.Equal(got, want) {
		t.Errorf("sent %q, expected %q", got, want)
	}
}

func TestLoadSaysWhatOllamaSaid(t *testing.T) {
	source, _ := engine(t, map[string]string{"/api/generate": "testdata/notfound.json"}, http.StatusNotFound)
	err := source.Load(context.Background(), "nosuch:1b")
	if err == nil || !strings.HasSuffix(err.Error(), "status 404: model 'nosuch:1b' not found") {
		t.Errorf("error %v; expected Ollama's own message", err)
	}
}
