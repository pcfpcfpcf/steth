package geode

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/pcfpcfpcf/steth/internal/engine/control"
)

// The answers follow the contract agreed with geode for runtime loads.
const (
	available = `{"object":"list","data":[
		{"id":"a.gguf","path":"/m/a.gguf","bytes":10,"served":true},
		{"id":"b.gguf","path":"/m/b.gguf","bytes":20,"served":false},
		{"id":"c.gguf","path":"/m/sub/c.gguf","bytes":30,"served":false}]}`
	served = `{"object":"list","data":[
		{"id":"A","object":"model","state":"ready","path":"/m/a.gguf"},
		{"id":"C","object":"model","state":"loading","path":"/m/sub/c.gguf"}]}`
)

func controlled(t *testing.T, answers map[string]string) (*Source, *[]string) {
	posted := &[]string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			body, _ := io.ReadAll(r.Body)
			*posted = append(*posted, r.URL.Path+" "+string(body))
			w.WriteHeader(http.StatusAccepted)
			io.WriteString(w, `{"id":"B","state":"loading"}`)
			return
		}
		answer, ok := answers[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		io.WriteString(w, answer)
	}))
	t.Cleanup(server.Close)
	return New(server.URL, server.Client()), posted
}

func TestFilesSayWhatEachIsNow(t *testing.T) {
	s, _ := controlled(t, map[string]string{"/models/available": available, "/v1/models": served})
	files, err := s.Files(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := []control.File{
		{Path: "/m/a.gguf", Model: "A", State: "ready"},
		{Path: "/m/b.gguf"},
		{Path: "/m/sub/c.gguf", Model: "C", State: "loading"},
	}
	if !reflect.DeepEqual(files, want) {
		t.Errorf("got %+v\nexpected %+v", files, want)
	}
	ids, err := s.Available(context.Background())
	if want := []string{"A", "C", "/m/b.gguf"}; err != nil || !reflect.DeepEqual(ids, want) {
		t.Errorf("available %q, %v; expected the served ids, then the files not loaded, %q", ids, err, want)
	}
}

func TestLoadTakesAPathUnloadAnID(t *testing.T) {
	s, posted := controlled(t, map[string]string{})
	if err := s.Load(context.Background(), "/m/b.gguf"); err != nil {
		t.Fatal(err)
	}
	if err := s.Unload(context.Background(), "A"); err != nil {
		t.Fatal(err)
	}
	if want := []string{`/models/load {"model":"/m/b.gguf"}`, `/models/unload {"model":"A"}`}; !reflect.DeepEqual(*posted, want) {
		t.Errorf("posted %q; expected %q", *posted, want)
	}
}

// A geode whose /v1/models names no paths still says which files it serves.
func TestServedFileWithoutItsModel(t *testing.T) {
	s, _ := controlled(t, map[string]string{"/models/available": available,
		"/v1/models": `{"object":"list","data":[{"id":"A","object":"model"}]}`})
	files, err := s.Files(context.Background())
	if err != nil || files[0] != (control.File{Path: "/m/a.gguf", State: "ready"}) || files[1].State != "" {
		t.Errorf("files %+v, %v; expected a.gguf ready and b.gguf not loaded", files, err)
	}
	ids, _ := s.Available(context.Background())
	if want := []string{"A", "/m/b.gguf", "/m/sub/c.gguf"}; !reflect.DeepEqual(ids, want) {
		t.Errorf("available %q; expected %q", ids, want)
	}
}

// A geode from before runtime loads has no /models/available.
func TestOlderGeodeHasNoControls(t *testing.T) {
	s, _ := controlled(t, map[string]string{"/v1/models": served})
	if _, err := s.Available(context.Background()); !errors.Is(err, control.ErrNone) {
		t.Errorf("got %v; expected control.ErrNone", err)
	}
}
