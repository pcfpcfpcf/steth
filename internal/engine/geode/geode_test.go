package geode

import (
	"context"
	"encoding/json"
	"flag"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

var update = flag.Bool("update", false, "rewrite testdata/*.want.json from what the adapter reads")

// serve answers GET /v1/pool with a recorded response.
func serve(t *testing.T, fixture string) *httptest.Server {
	body, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/pool" {
			http.NotFound(w, r)
			return
		}
		w.Write(body)
	}))
	t.Cleanup(server.Close)
	return server
}

func TestReadMatchesSchema(t *testing.T) {
	server := serve(t, "testdata/idle.json")
	reading, err := New(server.URL, server.Client()).Read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	got, err := json.MarshalIndent(reading, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	const want = "testdata/idle.want.json"
	if *update {
		os.WriteFile(want, append(got, '\n'), 0o644)
	}
	expected, err := os.ReadFile(want)
	if err != nil {
		t.Fatal(err)
	}
	if string(expected) != string(got)+"\n" {
		t.Errorf("reading differs from %s:\n%s", want, got)
	}
}

func TestReadFailsOnBadStatus(t *testing.T) {
	server := serve(t, "testdata/idle.json")
	_, err := New(server.URL+"/nowhere", server.Client()).Read(context.Background())
	if err == nil {
		t.Fatal("expected an error for a 404")
	}
}
