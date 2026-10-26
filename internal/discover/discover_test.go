package discover

import (
	"context"
	"errors"
	"net"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/pcfpcfpcf/steth/internal/box"
	"github.com/pcfpcfpcf/steth/internal/collector"
	"github.com/pcfpcfpcf/steth/internal/state"
)

type process struct {
	cmdline string
	parent  int
}

// machine is a fake System: listeners, and the processes this user can see.
func machine(listeners []box.Listener, processes map[int]process) System {
	return System{
		Listeners: func() ([]box.Listener, error) { return listeners, nil },
		Cmdline: func(pid int) ([]string, error) {
			p, ok := processes[pid]
			if !ok {
				return nil, errors.New("no such process")
			}
			return strings.Fields(p.cmdline), nil
		},
		ParentPID: func(pid int) (int, error) { return processes[pid].parent, nil },
	}
}

func listener(port, pid int) box.Listener {
	return box.Listener{IP: net.IPv4(127, 0, 0, 1), Port: port, PID: pid}
}

type source struct{ engine, url string }

func (s *source) Engine() string                              { return s.engine }
func (s *source) Endpoint() string                            { return s.url }
func (s *source) Read(context.Context) (state.Reading, error) { return state.Reading{}, nil }

// engine recognises `name` in a command line and answers probes at the
// URLs in `answers`; it records every URL probed.
func engine(name string, ports []int, answers []string, probed *[]string) Engine {
	return Engine{
		Name:  name,
		Runs:  func(cmdline []string) bool { return cmdline[0] == name },
		Ports: ports,
		Probe: func(_ context.Context, _ *http.Client, url string) bool {
			*probed = append(*probed, url)
			return slices.Contains(answers, url)
		},
		New: func(url string, _ *http.Client) collector.Source { return &source{name, url} },
	}
}

func TestFind(t *testing.T) {
	var probed []string
	engines := []Engine{
		engine("llama", []int{9931}, []string{"http://127.0.0.1:9000", "http://127.0.0.1:41001"}, &probed),
		engine("ollama", []int{11434}, []string{"http://127.0.0.1:11434"}, &probed),
	}
	sys := machine(
		[]box.Listener{listener(9000, 10), listener(41001, 11), listener(11434, 0), listener(5432, 20), listener(6000, 0)},
		map[int]process{
			10: {cmdline: "llama serve", parent: 1},
			11: {cmdline: "llama serve --port 41001", parent: 10}, // started by the router
			20: {cmdline: "postgres -D /data", parent: 1},
		},
	)
	found, err := Find(context.Background(), sys, nil, engines)
	if err != nil {
		t.Fatal(err)
	}
	want := []Found{{"llama", "http://127.0.0.1:9000"}, {"ollama", "http://127.0.0.1:11434"}}
	if !slices.Equal(found, want) {
		t.Errorf("found %v, expected %v", found, want)
	}
	for _, url := range []string{"http://127.0.0.1:41001", "http://127.0.0.1:5432", "http://127.0.0.1:6000"} {
		if slices.Contains(probed, url) {
			t.Errorf("probed %s: a router's child, another program, or an unknown port", url)
		}
	}
}

func TestRegistryKeepsSourcesFoundAgain(t *testing.T) {
	var probed []string
	ollama := engine("ollama", []int{11434}, []string{"http://127.0.0.1:11434", "http://127.0.0.1:11435"}, &probed)
	fixed := &source{"ollama", "http://127.0.0.1:11435"}
	both := machine([]box.Listener{listener(11434, 0), listener(11435, 30)}, map[int]process{30: {cmdline: "ollama serve"}})
	r := NewRegistry([]collector.Source{fixed}, []Engine{ollama}, nil)
	if err := r.Refresh(context.Background(), both); err != nil {
		t.Fatal(err)
	}
	first := r.Sources()
	if len(first) != 2 || first[0] != fixed {
		t.Fatalf("sources %v; expected the fixed one, then 11434 once", first)
	}
	r.Refresh(context.Background(), both)
	if again := r.Sources(); again[1] != first[1] {
		t.Error("a source found again was replaced, losing what it remembers")
	}
	r.Refresh(context.Background(), machine(nil, nil))
	if gone := r.Sources(); len(gone) != 1 {
		t.Errorf("sources %v after every engine stopped; expected only the fixed one", gone)
	}
}
