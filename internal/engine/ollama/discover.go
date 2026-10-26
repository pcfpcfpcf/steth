package ollama

import (
	"context"
	"net/http"
	"path/filepath"
	"slices"

	"github.com/pcfpcfpcf/steth/internal/engine/fetch"
)

// Ports is where Ollama listens unless OLLAMA_HOST says otherwise.
var Ports = []int{11434}

// Runs is `ollama serve`; the runners it starts per model listen too, but
// speak only to it.
func Runs(cmdline []string) bool {
	return len(cmdline) > 1 && filepath.Base(cmdline[0]) == "ollama" && slices.Contains(cmdline[1:], "serve")
}

// Probe asks GET /api/version, which only Ollama answers with a version.
func Probe(ctx context.Context, client *http.Client, url string) bool {
	var v struct {
		Version string `json:"version"`
	}
	return fetch.JSON(ctx, client, url+"/api/version", &v) == nil && v.Version != ""
}
