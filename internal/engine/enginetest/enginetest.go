// Package enginetest serves an engine's recorded responses and compares
// what an adapter reads from them with the expected schema JSON.
package enginetest

import (
	"encoding/json"
	"flag"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
)

var update = flag.Bool("update", false, "rewrite the expected JSON from what the adapter reads")

// Server answers each path it was given with the file it names, and any
// other path with 404. A path with a query matches only that query; one
// without matches any.
type Server struct {
	*httptest.Server
	t      *testing.T
	mu     sync.Mutex
	bodies map[string][]byte
}

// Serve starts a Server answering `fixtures`, path to file.
func Serve(t *testing.T, fixtures map[string]string) *Server {
	s := &Server{t: t, bodies: map[string][]byte{}}
	for path, file := range fixtures {
		s.Answer(path, file)
	}
	s.Server = httptest.NewServer(http.HandlerFunc(s.serve))
	t.Cleanup(s.Close)
	return s
}

// Answer makes `path` answer with `file` from now on, as an engine's
// counters move between two reads.
func (s *Server) Answer(path, file string) {
	body, err := os.ReadFile(file)
	if err != nil {
		s.t.Fatal(err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.bodies[path] = body
}

func (s *Server) serve(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	body, ok := s.bodies[r.URL.RequestURI()]
	if !ok {
		body, ok = s.bodies[r.URL.Path]
	}
	s.mu.Unlock()
	if !ok {
		http.NotFound(w, r)
		return
	}
	w.Write(body)
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
