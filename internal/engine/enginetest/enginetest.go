// Package enginetest serves an engine's recorded responses and compares
// what an adapter reads from them with the expected schema JSON.
package enginetest

import (
	"encoding/json"
	"flag"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

var update = flag.Bool("update", false, "rewrite the expected JSON from what the adapter reads")

// Serve answers each path in `fixtures` with the file it names, and any
// other path with 404. A path with a query matches only that query; one
// without matches any.
func Serve(t *testing.T, fixtures map[string]string) *httptest.Server {
	bodies := map[string][]byte{}
	for path, file := range fixtures {
		body, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		bodies[path] = body
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, ok := bodies[r.URL.RequestURI()]
		if !ok {
			body, ok = bodies[r.URL.Path]
		}
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Write(body)
	}))
	t.Cleanup(server.Close)
	return server
}

// Match fails the test unless `got`, as indented JSON, equals the file
// `want`; with -update it rewrites `want` first.
func Match(t *testing.T, got any, want string) {
	t.Helper()
	encoded, err := json.MarshalIndent(got, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	encoded = append(encoded, '\n')
	if *update {
		if err := os.WriteFile(want, encoded, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	expected, err := os.ReadFile(want)
	if err != nil {
		t.Fatal(err)
	}
	if string(expected) != string(encoded) {
		t.Errorf("differs from %s:\n%s", want, encoded)
	}
}
