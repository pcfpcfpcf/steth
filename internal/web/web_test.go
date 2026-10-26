package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/pcfpcfpcf/steth/internal/state"
)

func get(path string) *httptest.ResponseRecorder {
	h := Handler(func() state.State {
		return state.State{Schema: state.Schema, Models: []state.Model{{ID: "m"}}}
	})
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
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

func TestUnknownPathIsNotFound(t *testing.T) {
	if w := get("/nowhere"); w.Code != http.StatusNotFound {
		t.Errorf("status %d, expected 404", w.Code)
	}
}
