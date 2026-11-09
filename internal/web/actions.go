package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"slices"
	"time"

	"github.com/pcfpcfpcf/steth/internal/chat"
	"github.com/pcfpcfpcf/steth/internal/engine/control"
	"github.com/pcfpcfpcf/steth/internal/installed"
	"github.com/pcfpcfpcf/steth/internal/launch"
	"github.com/pcfpcfpcf/steth/internal/library"
	"github.com/pcfpcfpcf/steth/internal/state"
)

// An engine has this long to list, load or unload a model: loading reads
// the weights.
const controlTimeout = 5 * time.Minute

// Source is an engine steth reads; its adapter may offer control.Controls.
type Source interface {
	Engine() string
	Endpoint() string
}

// Actions is what the page may do, and with what.
type Actions struct {
	// Sources are the engines steth reads now; actions reach no other.
	Sources func() []Source
	// Launcher starts and stops engines from launch profiles; nil without
	// any.
	Launcher *launch.Launcher
	// Installed are the engines whose programs are on this machine.
	Installed []Installed
	// Places are where model files are on this machine.
	Places *library.Places
	// Client sends chats and controls; it needs no timeout, as a chat ends
	// when the page cancels it.
	Client *http.Client

	current func() state.State
}

func (a *Actions) register(mux *http.ServeMux) {
	if a.Places == nil {
		a.Places, _ = library.Open(nil, "") // kept in memory; it cannot fail
	}
	mux.HandleFunc("GET /api/v1/controls", guard(a.controls))
	mux.HandleFunc("POST /api/v1/load", guard(a.load))
	mux.HandleFunc("POST /api/v1/unload", guard(a.unload))
	mux.HandleFunc("POST /api/v1/chat", guard(a.chat))
	mux.HandleFunc("GET /api/v1/engines", guard(a.engines))
	mux.HandleFunc("GET /api/v1/launch", guard(a.launchStatus))
	mux.HandleFunc("POST /api/v1/launch/start", guard(a.launchStart))
	mux.HandleFunc("POST /api/v1/launch/stop", guard(a.launchStop))
	mux.HandleFunc("GET /api/v1/library", guard(a.libraryList))
	mux.HandleFunc("POST /api/v1/library/add", guard(a.placeAdd))
	mux.HandleFunc("POST /api/v1/library/remove", guard(a.placeRemove))
	mux.HandleFunc("GET /api/v1/library/browse", guard(a.browse))
}

// Installed is an engine's program on this machine and the launch
// profiles that start it.
type Installed struct {
	installed.Found
	Profiles []string `json:"profiles"`
}

// controllable is one engine with controls, and the models it could load
// or unload now.
type controllable struct {
	Engine   string   `json:"engine"`
	Endpoint string   `json:"endpoint"`
	Models   []string `json:"models"`
	// Files are the model files an engine that loads by file may open,
	// and what each is now; absent for an engine that loads by name, and
	// empty, not absent, for one that may open none (omitzero, as
	// omitempty would drop the empty list too).
	Files []control.File `json:"files,omitzero"`
	Error string         `json:"error,omitempty"`
}

// controls lists every engine whose adapter has controls and whose server
// answers with them; one without is left out, so the page shows no button
// that would fail.
func (a *Actions) controls(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), controlTimeout)
	defer cancel()
	out := []controllable{}
	for _, s := range a.Sources() {
		c, ok := s.(control.Controls)
		if !ok {
			continue
		}
		models, err := c.Available(ctx)
		if errors.Is(err, control.ErrNone) {
			continue
		}
		entry := controllable{Engine: s.Engine(), Endpoint: s.Endpoint(), Models: models}
		if entry.Models == nil {
			entry.Models = []string{}
		}
		if f, ok := s.(control.Files); ok && err == nil {
			entry.Files, err = f.Files(ctx)
			if entry.Files == nil {
				entry.Files = []control.File{}
			}
		}
		if err != nil {
			entry.Error = err.Error()
		}
		out = append(out, entry)
	}
	reply(w, out)
}

// target names a model on an engine steth reads.
type target struct {
	Engine   string `json:"engine"`
	Endpoint string `json:"endpoint"`
	Model    string `json:"model"`
}

// controlsOf is the controls of the source the target names; an address
// steth does not read is refused, so the page cannot send steth elsewhere.
func (a *Actions) controlsOf(t target) (control.Controls, error) {
	at := slices.IndexFunc(a.Sources(), func(s Source) bool { return s.Engine() == t.Engine && s.Endpoint() == t.Endpoint })
	if at < 0 {
		return nil, fmt.Errorf("steth reads no %s at %s", t.Engine, t.Endpoint)
	}
	c, ok := a.Sources()[at].(control.Controls)
	if !ok {
		return nil, fmt.Errorf("%s has no load or unload controls", t.Engine)
	}
	return c, nil
}

func (a *Actions) load(w http.ResponseWriter, r *http.Request) {
	a.control(w, r, "loaded", "loading", control.Controls.Load)
}

func (a *Actions) unload(w http.ResponseWriter, r *http.Request) {
	a.control(w, r, "unloaded", "unloading", control.Controls.Unload)
}

// control runs one load or unload through the engine named in the body.
// done is the reply when the engine answers once finished, begun when it
// answers once started.
func (a *Actions) control(w http.ResponseWriter, r *http.Request, done, begun string,
	act func(control.Controls, context.Context, string) error) {
	var t target
	if err := decode(w, r, &t); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	c, err := a.controlsOf(t)
	if err != nil {
		fail(w, http.StatusNotFound, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), controlTimeout)
	defer cancel()
	if err := act(c, ctx, t.Model); err != nil {
		fail(w, http.StatusBadGateway, err)
		return
	}
	if _, ok := c.(control.Begins); ok {
		done = begun
	}
	log.Printf("steth: %s %s on %s at %s", done, t.Model, t.Engine, t.Endpoint)
	reply(w, map[string]string{"model": t.Model, "done": done})
}

// chatRequest is a target and the conversation to send it.
type chatRequest struct {
	target
	chat.Request
}

// chat streams one reply from a model steth reads, then its numbers as an
// event of its own: `event: steth`, or `event: error` if the stream broke.
func (a *Actions) chat(w http.ResponseWriter, r *http.Request) {
	var req chatRequest
	if err := decode(w, r, &req); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	if !slices.ContainsFunc(a.current().Models, func(m state.Model) bool {
		return m.Engine == req.Engine && m.Endpoint == req.Endpoint && m.ID == req.Model
	}) {
		fail(w, http.StatusNotFound, fmt.Errorf("steth reads no model %q on %s at %s", req.Model, req.Engine, req.Endpoint))
		return
	}
	stream := &eventStream{w: w}
	flush := func() { http.NewResponseController(w).Flush() }
	numbers, err := chat.Send(r.Context(), a.Client, chat.Target{Engine: req.Engine, Endpoint: req.Endpoint, Model: req.Model},
		req.Request, stream, flush)
	switch {
	case err != nil && !stream.started:
		fail(w, http.StatusBadGateway, err)
	case err != nil:
		stream.event("error", map[string]string{"error": err.Error()})
	default:
		stream.event("steth", numbers)
	}
	flush()
}

// eventStream writes an event stream, its headers at the first write, so a
// request refused before any event can still be answered as JSON.
type eventStream struct {
	w       http.ResponseWriter
	started bool
}

func (s *eventStream) Write(p []byte) (int, error) {
	if !s.started {
		s.started = true
		s.w.Header().Set("Content-Type", "text/event-stream")
		s.w.Header().Set("Cache-Control", "no-store")
	}
	return s.w.Write(p)
}

func (s *eventStream) event(name string, v any) {
	data, err := json.Marshal(v)
	if err != nil {
		log.Printf("event %s not sent: %v", name, err)
		return
	}
	fmt.Fprintf(s, "event: %s\ndata: %s\n\n", name, data)
}

func (a *Actions) engines(w http.ResponseWriter, r *http.Request) {
	if a.Installed == nil {
		reply(w, []Installed{})
		return
	}
	reply(w, a.Installed)
}

func (a *Actions) launchStatus(w http.ResponseWriter, r *http.Request) {
	if a.Launcher == nil {
		reply(w, []launch.Status{})
		return
	}
	reply(w, a.Launcher.Status())
}

// launchRequest names a profile and, for one with {model}, the model.
type launchRequest struct {
	Name  string `json:"name"`
	Model string `json:"model"`
}

func (a *Actions) launchStart(w http.ResponseWriter, r *http.Request) {
	a.launch(w, r, "started", func(req launchRequest) error { return a.Launcher.Start(req.Name, req.Model) })
}

func (a *Actions) launchStop(w http.ResponseWriter, r *http.Request) {
	a.launch(w, r, "stopped", func(req launchRequest) error { return a.Launcher.Stop(req.Name) })
}

func (a *Actions) launch(w http.ResponseWriter, r *http.Request, done string, act func(launchRequest) error) {
	if a.Launcher == nil {
		fail(w, http.StatusNotFound, errors.New("no launch profiles: steth found no engine's program; name one with --engine ENGINE=PATH, or give --config FILE"))
		return
	}
	var req launchRequest
	if err := decode(w, r, &req); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	if err := act(req); err != nil {
		fail(w, http.StatusConflict, err)
		return
	}
	log.Printf("steth: %s %s %s", done, req.Name, req.Model)
	reply(w, map[string]string{"name": req.Name, "done": done})
}
