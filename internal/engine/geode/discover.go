package geode

import (
	"context"
	"net/http"
	"path/filepath"
	"slices"

	"github.com/pcfpcfpcf/steth/internal/engine/fetch"
)

// Ports is where `geode serve` listens unless given --port.
var Ports = []int{8080}

// Runs is `geode serve`.
func Runs(cmdline []string) bool {
	return len(cmdline) > 1 && filepath.Base(cmdline[0]) == "geode" && slices.Contains(cmdline[1:], "serve")
}

// Probe asks GET /v1/pool, which geode answers as an object "pool".
func Probe(ctx context.Context, client *http.Client, url string) bool {
	var p struct {
		Object string `json:"object"`
	}
	return fetch.JSON(ctx, client, url+"/v1/pool", &p) == nil && p.Object == "pool"
}
