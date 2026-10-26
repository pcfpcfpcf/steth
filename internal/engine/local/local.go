// Package local finds the process behind a server on this machine, so an
// adapter can read what the server's API does not say: its memory and its
// command line.
package local

import (
	"context"
	"net"
	"net/url"
	"strconv"

	"github.com/pcfpcfpcf/steth/internal/box"
)

// A port whose process could not be seen is looked up again after this many
// lookups: it may be another user's for good, and finding out scans every
// process.
const lookupEvery = 10

// Processes is what is read of the machine's processes; Box reads the real
// ones.
type Processes interface {
	PIDAt(port int) int
	Cmdline(pid int) ([]string, error)
	Memory(ctx context.Context, pid int) (map[string]uint64, error)
}

type Box struct{}

// PIDAt is the process listening on `port`, 0 where none can be seen.
func (Box) PIDAt(port int) int {
	listeners, err := box.Listeners()
	if err != nil {
		return 0
	}
	for _, l := range listeners {
		if l.Port == port && l.PID != 0 {
			return l.PID
		}
	}
	return 0
}

func (Box) Cmdline(pid int) ([]string, error) { return box.Cmdline(pid) }

func (Box) Memory(ctx context.Context, pid int) (map[string]uint64, error) {
	return box.ProcessMemory(ctx, pid)
}

// Server is the process behind one port.
type Server struct {
	PID  int
	Args []string
}

// Finder remembers the server process behind each port. It is used from one
// read at a time, so it needs no lock.
type Finder struct {
	Processes Processes
	runs      func(cmdline []string) bool
	known     map[int]*entry
}

type entry struct {
	server  *Server
	retryIn int
}

// NewFinder finds servers whose command line `runs` accepts.
func NewFinder(runs func(cmdline []string) bool) *Finder {
	return &Finder{Processes: Box{}, runs: runs, known: map[int]*entry{}}
}

// At is the server listening on `port`, nil where none can be seen; one
// gone since the last lookup is looked up again.
func (f *Finder) At(port int) *Server {
	if port == 0 {
		return nil
	}
	e := f.known[port]
	if e == nil {
		e = &entry{}
		f.known[port] = e
	}
	if e.server != nil {
		if args, err := f.Processes.Cmdline(e.server.PID); err == nil && f.runs(args) {
			return e.server
		}
		e.server = nil
	}
	if e.retryIn > 0 {
		e.retryIn--
		return nil
	}
	e.retryIn = lookupEvery
	pid := f.Processes.PIDAt(port)
	if pid == 0 {
		return nil
	}
	args, err := f.Processes.Cmdline(pid)
	if err != nil || !f.runs(args) {
		return nil
	}
	e.server, e.retryIn = &Server{PID: pid, Args: args}, 0
	return e.server
}

// Port is the port of `endpoint` where it is on this machine, else 0.
func Port(endpoint string) int {
	u, err := url.Parse(endpoint)
	if err != nil {
		return 0
	}
	host := u.Hostname()
	if ip := net.ParseIP(host); host != "localhost" && (ip == nil || !ip.IsLoopback()) {
		return 0
	}
	port, _ := strconv.Atoi(u.Port())
	return port
}
