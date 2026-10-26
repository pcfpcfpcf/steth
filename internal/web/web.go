// Package web serves the page and the state it draws.
package web

import (
	_ "embed"
	"encoding/json"
	"log"
	"net/http"

	"github.com/pcfpcfpcf/steth/internal/state"
)

//go:embed page.html
var page []byte

// Handler serves GET / (the page) and GET /api/v1/state (what `current`
// returns, as JSON).
func Handler(current func() state.State) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(page)
	})
	mux.HandleFunc("GET /api/v1/state", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		if err := json.NewEncoder(w).Encode(current()); err != nil {
			log.Printf("state not sent: %v", err)
		}
	})
	return mux
}
