package llamacpp

import (
	"context"
	"errors"
	"maps"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/pcfpcfpcf/steth/internal/engine/control"
	"github.com/pcfpcfpcf/steth/internal/engine/enginetest"
)

var _ control.Controls = (*Source)(nil)

// engine answers each path with its file, a GET with 200 and a POST with
// `posted`, and keeps every POST it was sent.
func engine(t *testing.T, answers map[string]string, posted int) (*Source, func() []string) {
	server := enginetest.Serve(t, answers)
	server.AnswerPosts(posted)
	return New(server.URL, server.Client()), server.Posted
}

var router = map[string]string{
	"/props":         "testdata/router.props.json",
	"/models":        "testdata/router.available.models.json",
	"/models/load":   "testdata/router.load.json",
	"/models/unload": "testdata/router.load.json",
}

// Recorded from a router over a directory of one model, which also lists a
// model in the Hugging Face cache.
func TestRouterLoadsAndUnloads(t *testing.T) {
	ctx := context.Background()
	source, sent := engine(t, router, http.StatusOK)
	got, err := source.Available(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"Qwen3.5-0.8B-Q8_0", "mradermacher/Tiny-Moe-GGUF:Q8_0"}; !slices.Equal(got, want) {
		t.Errorf("available %q, expected %q", got, want)
	}
	if err := source.Load(ctx, "Qwen3.5-0.8B-Q8_0"); err != nil {
		t.Fatal(err)
	}
	if err := source.Unload(ctx, "Qwen3.5-0.8B-Q8_0"); err != nil {
		t.Fatal(err)
	}
	want := []string{`/models/load {"model":"Qwen3.5-0.8B-Q8_0"}`, `/models/unload {"model":"Qwen3.5-0.8B-Q8_0"}`}
	if got := sent(); !slices.Equal(got, want) {
		t.Errorf("sent %q, expected %q", got, want)
	}
}

func TestRouterSaysWhyItRefused(t *testing.T) {
	for file, c := range map[string]struct {
		status int
		want   string
	}{
		"testdata/router.load.running.json": {http.StatusBadRequest, "status 400: model is already running"},
		"testdata/router.load.unknown.json": {http.StatusNotFound, "status 404: File Not Found"},
	} {
		answers := maps.Clone(router)
		answers["/models/load"] = file
		source, _ := engine(t, answers, c.status)
		err := source.Load(context.Background(), "Qwen3.5-0.8B-Q8_0")
		if err == nil || !strings.HasSuffix(err.Error(), c.want) {
			t.Errorf("%s: error %v; expected it to end %q", file, err, c.want)
		}
	}
}

// A server of one model has no controls and is sent nothing.
func TestOneModelHasNoControls(t *testing.T) {
	ctx := context.Background()
	answers := maps.Clone(router)
	answers["/props"] = "testdata/live.props.json"
	source, sent := engine(t, answers, http.StatusOK)
	if _, err := source.Available(ctx); !errors.Is(err, control.ErrNone) {
		t.Errorf("available: %v; expected ErrNone", err)
	}
	if err := source.Load(ctx, "m"); !errors.Is(err, control.ErrNone) {
		t.Errorf("load: %v; expected ErrNone", err)
	}
	if err := source.Unload(ctx, "m"); !errors.Is(err, control.ErrNone) {
		t.Errorf("unload: %v; expected ErrNone", err)
	}
	if got := sent(); len(got) > 0 {
		t.Errorf("sent %q to a server of one model", got)
	}
}
