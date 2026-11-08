// Package web serves the page and the state it draws, and, to the person at
// the box, the actions the page takes: chat with a model, load and unload
// one through its engine, and start and stop engines from launch profiles.
package web

import (
	"bytes"
	"crypto/sha256"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"log"
	"net/http"

	"github.com/pcfpcfpcf/steth/internal/state"
)

//go:embed page.html
var page []byte

// pagePolicy lets the page run its own script and nothing else, and talk
// only to steth. Styles stay inline: the page sets colours per element.
var pagePolicy = "default-src 'none'; script-src 'sha256-" + scriptHash(page) + "'; " +
	"style-src 'unsafe-inline'; connect-src 'self'; img-src 'self' data:; " +
	"base-uri 'none'; form-action 'none'; frame-ancestors 'none'"

// scriptHash is the base64 SHA-256 of the page's one inline script, as CSP
// names it.
func scriptHash(html []byte) string {
	_, rest, _ := bytes.Cut(html, []byte("<script>"))
	script, _, _ := bytes.Cut(rest, []byte("</script>"))
	sum := sha256.Sum256(script)
	return base64.StdEncoding.EncodeToString(sum[:])
}

// Handler serves GET / (the page) and GET /api/v1/state (what `current`
// returns, as JSON). With `actions` it also serves the action routes, each
// answered only to steth's own page on a loopback address; without, steth
// only monitors.
func Handler(current func() state.State, actions *Actions) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Content-Security-Policy", pagePolicy)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
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
	if actions != nil {
		actions.current = current
		actions.register(mux)
	}
	return mux
}
