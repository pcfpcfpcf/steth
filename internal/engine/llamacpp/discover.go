package llamacpp

import (
	"context"
	"net/http"
	"path/filepath"

	"github.com/pcfpcfpcf/steth/internal/engine/fetch"
)

// Ports is where `llama serve` listens by default (9931), and llama-server
// did before it (8080).
var Ports = []int{9931, 8080}

// Runs is `llama serve` (or its alias `llama server`), or the older
// llama-server binary.
func Runs(cmdline []string) bool {
	if len(cmdline) == 0 {
		return false
	}
	switch filepath.Base(cmdline[0]) {
	case "llama-server":
		return true
	case "llama":
		return len(cmdline) > 1 && (cmdline[1] == "serve" || cmdline[1] == "server")
	}
	return false
}

// Probe asks GET /props, which a llama.cpp server, router or not, answers
// with its build.
func Probe(ctx context.Context, client *http.Client, url string) bool {
	var p struct {
		BuildInfo string `json:"build_info"`
	}
	return fetch.JSON(ctx, client, url+"/props", &p) == nil && p.BuildInfo != ""
}
