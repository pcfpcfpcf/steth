package vllm

import (
	"context"
	"net/http"
	"path/filepath"

	"github.com/pcfpcfpcf/steth/internal/engine/fetch"
)

// Ports is where vLLM listens unless given --port.
var Ports = []int{8000}

// Runs is `vllm serve`, as a script or through python, or the OpenAI server
// module run directly.
func Runs(cmdline []string) bool {
	for i, arg := range cmdline {
		if arg == "vllm.entrypoints.openai.api_server" {
			return true
		}
		if filepath.Base(arg) == "vllm" && i+1 < len(cmdline) && cmdline[i+1] == "serve" {
			return true
		}
	}
	return false
}

// Probe asks GET /v1/models, whose cards vLLM marks as its own.
func Probe(ctx context.Context, client *http.Client, url string) bool {
	var models wireModels
	if fetch.JSON(ctx, client, url+"/v1/models", &models) != nil || len(models.Data) == 0 {
		return false
	}
	return models.Data[0].OwnedBy == "vllm"
}
