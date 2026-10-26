package local

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// machine is a fake process table that counts port lookups.
type machine struct {
	byPort  map[int]int
	cmdline map[int]string
	lookups int
}

func (m *machine) PIDAt(port int) int { m.lookups++; return m.byPort[port] }

func (m *machine) Cmdline(pid int) ([]string, error) {
	if c, ok := m.cmdline[pid]; ok {
		return strings.Fields(c), nil
	}
	return nil, errors.New("no such process")
}

func (m *machine) Memory(context.Context, int) (map[string]uint64, error) { return nil, nil }

func runsServe(cmdline []string) bool { return len(cmdline) > 1 && cmdline[1] == "serve" }

func TestFinder(t *testing.T) {
	m := &machine{byPort: map[int]int{9931: 7, 8080: 0}, cmdline: map[int]string{7: "llama serve"}}
	f := NewFinder(runsServe)
	f.Processes = m
	if s := f.At(9931); s == nil || s.PID != 7 {
		t.Fatalf("server %+v; expected pid 7", s)
	}
	f.At(9931)
	if m.lookups != 1 {
		t.Errorf("%d lookups for a process still there; expected 1", m.lookups)
	}
	delete(m.cmdline, 7)
	if s := f.At(9931); s != nil {
		t.Error("a process gone is still returned")
	}

	m.lookups = 0
	for i := 0; i < lookupEvery+1; i++ {
		f.At(8080)
	}
	if m.lookups != 1 {
		t.Errorf("%d lookups of a port no process can be seen on, in %d reads; expected 1", m.lookups, lookupEvery+1)
	}
	f.At(8080)
	if m.lookups != 2 {
		t.Errorf("not looked up again after %d reads", lookupEvery)
	}
}

func TestPort(t *testing.T) {
	for endpoint, want := range map[string]int{
		"http://127.0.0.1:9931":    9931,
		"http://localhost:11434":   11434,
		"http://[::1]:8000":        8000,
		"http://192.168.1.20:8000": 0,
		"http://gpu-box:8000":      0,
	} {
		if got := Port(endpoint); got != want {
			t.Errorf("%s: %d, expected %d", endpoint, got, want)
		}
	}
}
