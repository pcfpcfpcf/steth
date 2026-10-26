// Package discover finds the inference engines listening on this machine:
// a listening socket whose process an engine recognises as its own, or,
// where the process is another user's, a well-known port, confirmed in
// either case by asking the port as that engine.
package discover

import (
	"context"
	"net/http"
	"slices"
	"sync"

	"github.com/pcfpcfpcf/steth/internal/box"
	"github.com/pcfpcfpcf/steth/internal/collector"
)

// Engine is what discovery needs of one engine; its adapter supplies it.
type Engine struct {
	Name string
	// Runs says whether a process with this command line serves this
	// engine's API.
	Runs func(cmdline []string) bool
	// Ports are where the engine listens unless told otherwise; they are
	// probed when the listening process cannot be seen.
	Ports []int
	// Probe says whether url answers as this engine.
	Probe func(ctx context.Context, client *http.Client, url string) bool
	New   func(url string, client *http.Client) collector.Source
}

// Found is an engine listening at URL.
type Found struct {
	Engine string
	URL    string
}

// System is what discovery reads of the machine; box reads the real one.
type System struct {
	Listeners func() ([]box.Listener, error)
	Cmdline   func(pid int) ([]string, error)
	ParentPID func(pid int) (int, error)
}

// Box is the machine as the box package reads it.
var Box = System{Listeners: box.Listeners, Cmdline: box.Cmdline, ParentPID: box.ParentPID}

// Find lists the engines listening now, one per URL.
func Find(ctx context.Context, sys System, client *http.Client, engines []Engine) ([]Found, error) {
	listeners, err := sys.Listeners()
	if err != nil {
		return nil, err
	}
	listening := map[int]bool{}
	for _, l := range listeners {
		if l.PID != 0 {
			listening[l.PID] = true
		}
	}
	var found []Found
	seen := map[string]bool{}
	for _, l := range listeners {
		url := l.URL()
		if seen[url] {
			continue
		}
		for _, e := range candidates(sys, l, listening, engines) {
			if e.Probe(ctx, client, url) {
				found = append(found, Found{Engine: e.Name, URL: url})
				seen[url] = true
				break
			}
		}
	}
	return found, nil
}

// candidates are the engines worth asking at a listener: the one its
// process runs, or, for a process this user cannot see, those whose
// well-known port it is. A server started by another server of the same
// engine is skipped: the parent reports it, as llama.cpp's router does the
// servers it starts.
func candidates(sys System, l box.Listener, listening map[int]bool, engines []Engine) []Engine {
	if l.PID == 0 {
		var byPort []Engine
		for _, e := range engines {
			if slices.Contains(e.Ports, l.Port) {
				byPort = append(byPort, e)
			}
		}
		return byPort
	}
	cmdline, err := sys.Cmdline(l.PID)
	if err != nil {
		return nil
	}
	for _, e := range engines {
		if !e.Runs(cmdline) {
			continue
		}
		if parent, err := sys.ParentPID(l.PID); err == nil && listening[parent] {
			if parentCmdline, err := sys.Cmdline(parent); err == nil && e.Runs(parentCmdline) {
				return nil
			}
		}
		return []Engine{e}
	}
	return nil
}

// Registry is the sources to read: the ones named on the command line, and
// those discovery last found. A source found again is kept, with what it
// remembers between reads.
type Registry struct {
	fixed   []collector.Source
	engines []Engine
	client  *http.Client
	mu      sync.Mutex
	found   map[string]collector.Source
}

func NewRegistry(fixed []collector.Source, engines []Engine, client *http.Client) *Registry {
	return &Registry{fixed: fixed, engines: engines, client: client, found: map[string]collector.Source{}}
}

// Sources is the fixed sources, then those found, ordered by URL.
func (r *Registry) Sources() []collector.Source {
	r.mu.Lock()
	defer r.mu.Unlock()
	list := slices.Clone(r.fixed)
	urls := make([]string, 0, len(r.found))
	for url := range r.found {
		urls = append(urls, url)
	}
	slices.Sort(urls)
	for _, url := range urls {
		list = append(list, r.found[url])
	}
	return list
}

// Refresh looks again, keeping the sources still there, adding new ones and
// dropping the gone. A URL a fixed source already reads is not added.
func (r *Registry) Refresh(ctx context.Context, sys System) error {
	found, err := Find(ctx, sys, r.client, r.engines)
	if err != nil {
		return err
	}
	byName := map[string]Engine{}
	for _, e := range r.engines {
		byName[e.Name] = e
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	next := map[string]collector.Source{}
	for _, f := range found {
		if slices.ContainsFunc(r.fixed, func(s collector.Source) bool { return s.Endpoint() == f.URL }) {
			continue
		}
		if kept, ok := r.found[f.URL]; ok && kept.Engine() == f.Engine {
			next[f.URL] = kept
			continue
		}
		next[f.URL] = byName[f.Engine].New(f.URL, r.client)
	}
	r.found = next
	return nil
}
