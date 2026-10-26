package llamacpp

import (
	"context"
	"net"
	"net/url"
	"slices"
	"strconv"

	"github.com/pcfpcfpcf/steth/internal/box"
)

// A port whose process could not be seen is looked up again after this many
// reads: it may be another user's for good, and finding out scans every
// process.
const lookupEvery = 10

// processes is what the adapter reads of a local server's own process; box
// reads the real ones.
type processes interface {
	pidAt(port int) int
	cmdline(pid int) ([]string, error)
	memory(ctx context.Context, pid int) (map[string]uint64, error)
}

type boxProcesses struct{}

func (boxProcesses) pidAt(port int) int {
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

func (boxProcesses) cmdline(pid int) ([]string, error) { return box.Cmdline(pid) }

func (boxProcesses) memory(ctx context.Context, pid int) (map[string]uint64, error) {
	return box.ProcessMemory(ctx, pid)
}

// server is the process behind one port, as far as it could be seen.
type server struct {
	pid     int
	args    launch
	retryIn int
}

// processAt is the process listening on `port`, nil where none can be seen;
// a process gone is looked up again.
func (s *Source) processAt(port int) *server {
	if port == 0 {
		return nil
	}
	known := s.servers[port]
	if known == nil {
		known = &server{}
		s.servers[port] = known
	}
	if known.pid != 0 {
		if args, err := s.proc.cmdline(known.pid); err == nil && Runs(args) {
			return known
		}
		*known = server{}
	}
	if known.retryIn > 0 {
		known.retryIn--
		return nil
	}
	pid := s.proc.pidAt(port)
	if pid == 0 {
		known.retryIn = lookupEvery
		return nil
	}
	args, err := s.proc.cmdline(pid)
	if err != nil || !Runs(args) {
		known.retryIn = lookupEvery
		return nil
	}
	*known = server{pid: pid, args: launch(args)}
	return known
}

// localPort is the port of `endpoint` where it is on this machine, else 0.
func localPort(endpoint string) int {
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

// launch is a server's command line.
type launch []string

// value is what follows the first of `names` given, as llama.cpp's own
// parser reads "--flag value".
func (l launch) value(names ...string) (string, bool) {
	for i, arg := range l[:max(len(l)-1, 0)] {
		if slices.Contains(names, arg) {
			return l[i+1], true
		}
	}
	return "", false
}

func (l launch) has(names ...string) bool {
	return slices.ContainsFunc(l, func(a string) bool { return slices.Contains(names, a) })
}

// port is where the server listens, 0 when its command line does not say.
func (l launch) port() int {
	v, _ := l.value("--port")
	port, _ := strconv.Atoi(v)
	return port
}

// sleeps is whether the server frees its model when idle.
func (l launch) sleeps() bool {
	v, given := l.value("--sleep-idle-seconds")
	seconds, err := strconv.Atoi(v)
	return given && err == nil && seconds > 0
}

// unifiedKV is whether every slot shares one KV cache: by default only when
// the number of slots is left to the server.
func (l launch) unifiedKV() bool {
	switch {
	case l.has("-kvu", "--kv-unified"):
		return true
	case l.has("-no-kvu", "--no-kv-unified"):
		return false
	}
	slots, given := l.value("-np", "--parallel")
	return !given || slots == "-1"
}
