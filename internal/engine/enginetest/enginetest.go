// Package enginetest serves an engine's recorded responses and compares
// what an adapter reads from them with the expected schema JSON.
package enginetest

import (
	"encoding/json"
	"flag"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"sync"
	"testing"
)

var update = flag.Bool("update", false, "rewrite the expected JSON from what the adapter reads")

// Server answers each path it was given with the file it names, and any
// other path with 404. A path with a query matches only that query; one
// without matches any. It keeps every POST it is sent.
type Server struct {
	*httptest.Server
	t      *testing.T
	mu     sync.Mutex
	bodies map[string][]byte
	// posted is the status a POST is answered with; sent, each POST's path
	// and body.
	posted int
	sent   []string
}

// Serve starts a Server answering `fixtures`, path to file.
func Serve(t *testing.T, fixtures map[string]string) *Server {
	s := &Server{t: t, bodies: map[string][]byte{}, posted: http.StatusOK}
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

// AnswerPosts makes every POST answer with `status`, as an engine that
// refuses one does.
func (s *Server) AnswerPosts(status int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.posted = status
}

// Posted is every POST the server was sent, as "path?query body", oldest
// first.
func (s *Server) Posted() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.sent)
}

func (s *Server) serve(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	body, ok := s.bodies[r.URL.RequestURI()]
	if !ok {
		body, ok = s.bodies[r.URL.Path]
	}
	status := http.StatusOK
	if ok && r.Method == http.MethodPost {
		sent, _ := io.ReadAll(r.Body)
		s.sent = append(s.sent, r.URL.RequestURI()+" "+string(sent))
		status = s.posted
	}
	s.mu.Unlock()
	if !ok {
		http.NotFound(w, r)
		return
	}
	w.WriteHeader(status)
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
