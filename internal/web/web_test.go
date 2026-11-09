package web

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pcfpcfpcf/steth/internal/engine/control"
	"github.com/pcfpcfpcf/steth/internal/installed"
	"github.com/pcfpcfpcf/steth/internal/library"
	"github.com/pcfpcfpcf/steth/internal/state"
)

func current() state.State {
	return state.State{Schema: state.Schema, Models: []state.Model{{ID: "m", Engine: "fake", Endpoint: "http://127.0.0.1:1"}}}
}

func get(path string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	Handler(current, nil).ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
	return w
}

func TestServesState(t *testing.T) {
	w := get("/api/v1/state")
	var s state.State
	if err := json.Unmarshal(w.Body.Bytes(), &s); err != nil {
		t.Fatal(err)
	}
	if w.Code != http.StatusOK || s.Schema != state.Schema || len(s.Models) != 1 {
		t.Errorf("status %d, body %s", w.Code, w.Body)
	}
}

func TestServesPage(t *testing.T) {
	w := get("/")
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "/api/v1/state") {
		t.Errorf("status %d; page does not read the state", w.Code)
	}
}

// The page's policy names its script's hash, so the script runs and no
// other does; a page whose script changed gets a new hash.
func TestPagePolicyNamesTheScript(t *testing.T) {
	policy := get("/").Header().Get("Content-Security-Policy")
	if !strings.Contains(policy, "'sha256-"+scriptHash(page)+"'") || strings.Contains(policy, "script-src 'unsafe-inline'") {
		t.Errorf("policy %q", policy)
	}
	if scriptHash([]byte("<script>a</script>")) == scriptHash([]byte("<script>b</script>")) {
		t.Error("two scripts share a hash")
	}
}

func TestUnknownPathIsNotFound(t *testing.T) {
	if w := get("/nowhere"); w.Code != http.StatusNotFound {
		t.Errorf("status %d, expected 404", w.Code)
	}
}

// fakeEngine is a source with controls that records what it was asked.
type fakeEngine struct {
	asked []string
	none  bool
}

func (f *fakeEngine) Engine() string   { return "fake" }
func (f *fakeEngine) Endpoint() string { return "http://127.0.0.1:1" }
func (f *fakeEngine) Available(context.Context) ([]string, error) {
	if f.none {
		return nil, control.ErrNone
	}
	return []string{"m", "n"}, nil
}
func (f *fakeEngine) Load(_ context.Context, model string) error {
	f.asked = append(f.asked, "load "+model)
	return nil
}
func (f *fakeEngine) Unload(_ context.Context, model string) error {
	if model == "busy" {
		return errors.New("the engine said no")
	}
	f.asked = append(f.asked, "unload "+model)
	return nil
}

// beginning is an engine that answers once a load has begun.
type beginning struct{ fakeEngine }

func (*beginning) Begins() {}

func TestLoadThatBeginsSaysSo(t *testing.T) {
	for _, c := range []struct {
		engine Source
		want   string
	}{
		{&fakeEngine{}, `"done":"loaded"`},
		{&beginning{}, `"done":"loading"`},
	} {
		h := Handler(current, &Actions{Sources: func() []Source { return []Source{c.engine} }, Client: http.DefaultClient})
		if w := act(h, http.MethodPost, "/api/v1/load", loadM, nil); w.Code != http.StatusOK || !strings.Contains(w.Body.String(), c.want) {
			t.Errorf("load: status %d %s; expected %s", w.Code, w.Body, c.want)
		}
	}
}

// plain is a source without controls.
type plain struct{}

func (plain) Engine() string   { return "plain" }
func (plain) Endpoint() string { return "http://127.0.0.1:2" }

func withActions(engine *fakeEngine) http.Handler {
	return Handler(current, &Actions{
		Sources: func() []Source { return []Source{plain{}, engine} },
		Client:  http.DefaultClient,
	})
}

// act sends an action as the page on 127.0.0.1:7474 would, with `edit`
// changing the request first.
func act(h http.Handler, method, path, body string, edit func(*http.Request)) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.Host = "127.0.0.1:7474"
	if method == http.MethodPost {
		r.Header.Set("Origin", "http://127.0.0.1:7474")
		r.Header.Set("Content-Type", "application/json")
	}
	if edit != nil {
		edit(r)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

const loadM = `{"engine":"fake","endpoint":"http://127.0.0.1:1","model":"m"}`

func TestGuardRefusesOtherPagesAndNames(t *testing.T) {
	engine := &fakeEngine{}
	h := withActions(engine)
	for name, edit := range map[string]func(*http.Request){
		"a foreign page": func(r *http.Request) { r.Header.Set("Origin", "http://evil.example") },
		"no origin":      func(r *http.Request) { r.Header.Del("Origin") },
		"a rebound name": func(r *http.Request) {
			r.Host = "evil.example:7474"
			r.Header.Set("Origin", "http://evil.example:7474")
		},
		"a LAN address":             func(r *http.Request) { r.Host = "192.168.1.5:7474"; r.Header.Set("Origin", "http://192.168.1.5:7474") },
		"an origin on another port": func(r *http.Request) { r.Header.Set("Origin", "http://127.0.0.1:8000") },
	} {
		if w := act(h, http.MethodPost, "/api/v1/load", loadM, edit); w.Code != http.StatusForbidden {
			t.Errorf("%s: status %d, expected 403", name, w.Code)
		}
	}
	if w := act(h, http.MethodGet, "/api/v1/controls", "", func(r *http.Request) { r.Host = "evil.example:7474" }); w.Code != http.StatusForbidden {
		t.Errorf("a rebound name reading controls: status %d, expected 403", w.Code)
	}
	if len(engine.asked) != 0 {
		t.Errorf("the engine was asked %v", engine.asked)
	}
	// The state reads as before, from anywhere.
	r := httptest.NewRequest(http.MethodGet, "/api/v1/state", nil)
	r.Host = "evil.example:7474"
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("state: status %d", w.Code)
	}
}

func TestGuardTakesLocalhost(t *testing.T) {
	engine := &fakeEngine{}
	w := act(withActions(engine), http.MethodPost, "/api/v1/load", loadM, func(r *http.Request) {
		r.Host = "localhost:7474"
		r.Header.Set("Origin", "http://localhost:7474")
	})
	if w.Code != http.StatusOK || len(engine.asked) != 1 {
		t.Errorf("status %d, asked %v", w.Code, engine.asked)
	}
}

func TestBodyMustBeSmallKnownJSON(t *testing.T) {
	h := withActions(&fakeEngine{})
	for name, c := range map[string]struct {
		body string
		edit func(*http.Request)
	}{
		"form":          {loadM, func(r *http.Request) { r.Header.Set("Content-Type", "application/x-www-form-urlencoded") }},
		"unknown field": {`{"engine":"fake","endpoint":"http://127.0.0.1:1","model":"m","url":"http://x"}`, nil},
		"too large":     {`{"engine":"` + strings.Repeat("a", bodyMax) + `"}`, nil},
	} {
		if w := act(h, http.MethodPost, "/api/v1/load", c.body, c.edit); w.Code != http.StatusBadRequest {
			t.Errorf("%s: status %d, expected 400", name, w.Code)
		}
	}
}

// Without actions -- steth bound off loopback -- no action route exists.
func TestNoActionsOffLoopback(t *testing.T) {
	h := Handler(current, nil)
	for _, path := range []string{"/api/v1/load", "/api/v1/chat", "/api/v1/launch/start"} {
		if w := act(h, http.MethodPost, path, loadM, nil); w.Code != http.StatusNotFound && w.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s: status %d, expected no route", path, w.Code)
		}
	}
}

func TestLoopback(t *testing.T) {
	for listen, want := range map[string]bool{
		"127.0.0.1:7474": true, "localhost:7474": true, "[::1]:7474": true, "127.0.0.2:1": true,
		"0.0.0.0:7474": false, ":7474": false, "192.168.1.5:7474": false, "box.lan:7474": false,
	} {
		if got := Loopback(listen); got != want {
			t.Errorf("%s: %v, expected %v", listen, got, want)
		}
	}
}

func TestControlsListOnlyEnginesThatHaveThem(t *testing.T) {
	w := act(withActions(&fakeEngine{}), http.MethodGet, "/api/v1/controls", "", nil)
	var got []controllable
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Engine != "fake" || len(got[0].Models) != 2 {
		t.Errorf("controls %s", w.Body)
	}
	w = act(withActions(&fakeEngine{none: true}), http.MethodGet, "/api/v1/controls", "", nil)
	if strings.TrimSpace(w.Body.String()) != "[]" {
		t.Errorf("a server without controls is listed: %s", w.Body)
	}
}

// fileEngine loads by file and may open none, as geode does started
// without --models-dir.
type fileEngine struct{ fakeEngine }

func (*fileEngine) Endpoint() string                              { return "http://127.0.0.1:3" }
func (*fileEngine) Files(context.Context) ([]control.File, error) { return nil, nil }

// The page tells an engine that loads by file from one that loads by name
// by whether it lists files at all, so an empty list must reach it.
func TestControlsListAnEmptyFileList(t *testing.T) {
	h := Handler(current, &Actions{
		Sources: func() []Source { return []Source{&fakeEngine{}, &fileEngine{}} },
		Client:  http.DefaultClient,
	})
	w := act(h, http.MethodGet, "/api/v1/controls", "", nil)
	var got []map[string]json.RawMessage
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil || len(got) != 2 {
		t.Fatalf("controls %s: %v", w.Body, err)
	}
	if files, ok := got[0]["files"]; ok {
		t.Errorf("an engine that loads by name lists files %s", files)
	}
	if files := string(got[1]["files"]); files != "[]" {
		t.Errorf("an engine that may open no file lists files %q, expected []", files)
	}
}

func TestLoadAndUnloadGoToTheEngine(t *testing.T) {
	engine := &fakeEngine{}
	h := withActions(engine)
	if w := act(h, http.MethodPost, "/api/v1/load", loadM, nil); w.Code != http.StatusOK {
		t.Errorf("load: status %d %s", w.Code, w.Body)
	}
	if w := act(h, http.MethodPost, "/api/v1/unload", strings.Replace(loadM, `"m"`, `"busy"`, 1), nil); w.Code != http.StatusBadGateway ||
		!strings.Contains(w.Body.String(), "the engine said no") {
		t.Errorf("a refused unload: status %d %s", w.Code, w.Body)
	}
	if strings.Join(engine.asked, ",") != "load m" {
		t.Errorf("asked %v", engine.asked)
	}
	for name, body := range map[string]string{
		"an address steth does not read": `{"engine":"fake","endpoint":"http://10.0.0.1:1","model":"m"}`,
		"an engine without controls":     `{"engine":"plain","endpoint":"http://127.0.0.1:2","model":"m"}`,
	} {
		if w := act(h, http.MethodPost, "/api/v1/load", body, nil); w.Code != http.StatusNotFound {
			t.Errorf("%s: status %d, expected 404", name, w.Code)
		}
	}
}

// A chat goes only to a model steth reads; the engine's events reach the
// page, then the reply's numbers.
func TestChatStreamsThenNumbers(t *testing.T) {
	engine := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, `data: {"choices":[{"delta":{"content":"hi"}}]}`+"\n\n")
		io.WriteString(w, `data: {"choices":[{"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":3,"completion_tokens":1}}`+"\n\n")
		io.WriteString(w, "data: [DONE]\n\n")
	}))
	defer engine.Close()
	h := Handler(func() state.State {
		return state.State{Models: []state.Model{{ID: "m", Engine: "fake", Endpoint: engine.URL}}}
	}, &Actions{Sources: func() []Source { return nil }, Client: engine.Client()})

	body := `{"engine":"fake","endpoint":"` + engine.URL + `","model":"m","messages":[{"role":"user","content":"hello"}]}`
	w := act(h, http.MethodPost, "/api/v1/chat", body, nil)
	out := w.Body.String()
	if w.Code != http.StatusOK || w.Header().Get("Content-Type") != "text/event-stream" ||
		!strings.Contains(out, `"content":"hi"`) || !strings.Contains(out, "event: steth\ndata: {") {
		t.Errorf("status %d, body %s", w.Code, out)
	}
	if w := act(h, http.MethodPost, "/api/v1/chat", strings.Replace(body, `"model":"m"`, `"model":"other"`, 1), nil); w.Code != http.StatusNotFound {
		t.Errorf("a model steth does not read: status %d", w.Code)
	}
}

func TestLaunchWithoutConfig(t *testing.T) {
	h := withActions(&fakeEngine{})
	if w := act(h, http.MethodGet, "/api/v1/launch", "", nil); strings.TrimSpace(w.Body.String()) != "[]" {
		t.Errorf("status without profiles: %s", w.Body)
	}
	if w := act(h, http.MethodPost, "/api/v1/launch/start", `{"name":"x"}`, nil); w.Code != http.StatusNotFound ||
		!strings.Contains(w.Body.String(), "--config") {
		t.Errorf("start without profiles: status %d %s", w.Code, w.Body)
	}
}

func TestEnginesListsThePrograms(t *testing.T) {
	if w := act(withActions(&fakeEngine{}), http.MethodGet, "/api/v1/engines", "", nil); strings.TrimSpace(w.Body.String()) != "[]" {
		t.Errorf("no program found: %s", w.Body)
	}
	h := Handler(current, &Actions{Sources: func() []Source { return nil }, Installed: []Installed{{
		Found:    installed.Found{Engine: "geode", Path: "/opt/geode", Named: true, Binary: installed.Binary{Name: "geode"}},
		Profiles: []string{"geode"},
	}}})
	w := act(h, http.MethodGet, "/api/v1/engines", "", nil)
	if want := `[{"engine":"geode","path":"/opt/geode","named":true,"profiles":["geode"]}]`; strings.TrimSpace(w.Body.String()) != want {
		t.Errorf("got %s\nexpected %s", w.Body, want)
	}
	if w := act(h, http.MethodGet, "/api/v1/engines", "", func(r *http.Request) { r.Host = "evil.example:7474" }); w.Code != http.StatusForbidden {
		t.Errorf("a rebound name: status %d; expected 403", w.Code)
	}
}

func TestLibraryPlacesAndBrowse(t *testing.T) {
	dir := t.TempDir()
	model := filepath.Join(dir, "m.gguf")
	if err := os.WriteFile(model, []byte("12345"), 0o600); err != nil {
		t.Fatal(err)
	}
	h := withActions(&fakeEngine{})
	body := func(path string) string {
		text, _ := json.Marshal(map[string]string{"path": path})
		return string(text)
	}
	var view libraryView
	w := act(h, http.MethodPost, "/api/v1/library/add", body(dir), nil)
	if err := json.Unmarshal(w.Body.Bytes(), &view); w.Code != http.StatusOK || err != nil {
		t.Fatalf("add: status %d %s", w.Code, w.Body)
	}
	real, _ := filepath.EvalSymlinks(model)
	if len(view.Places) != 1 || len(view.Files) != 1 || view.Files[0].Path != model || view.Files[0].Bytes != 5 || view.Files[0].Real != real {
		t.Errorf("after add: %+v", view)
	}
	if w := act(h, http.MethodPost, "/api/v1/library/add", body("relative"), nil); w.Code != http.StatusConflict {
		t.Errorf("a relative place: status %d %s", w.Code, w.Body)
	}
	if w := act(h, http.MethodPost, "/api/v1/library/add", body(dir), func(r *http.Request) { r.Header.Set("Origin", "http://evil.example") }); w.Code != http.StatusForbidden {
		t.Errorf("another page adding a place: status %d", w.Code)
	}
	var listing library.Listing
	w = act(h, http.MethodGet, "/api/v1/library/browse?dir="+url.QueryEscape(dir), "", nil)
	if err := json.Unmarshal(w.Body.Bytes(), &listing); err != nil || len(listing.Files) != 1 {
		t.Errorf("browse: status %d %s", w.Code, w.Body)
	}
	if w := act(h, http.MethodPost, "/api/v1/library/remove", body(dir), nil); w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"places":[]`) {
		t.Errorf("remove: status %d %s", w.Code, w.Body)
	}
}
