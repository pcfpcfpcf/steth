package box

// Tests that read this machine, whichever system it runs; CI runs them on
// Linux, macOS and Windows.

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUnspecifiedListenerIsReachedOnLoopback(t *testing.T) {
	if url := (Listener{IP: net.IPv4zero, Port: 9931}).URL(); url != "http://127.0.0.1:9931" {
		t.Errorf("got %s", url)
	}
	if url := (Listener{IP: net.IPv6unspecified, Port: 9931}).URL(); url != "http://[::1]:9931" {
		t.Errorf("got %s", url)
	}
}

// A listener this test opens is found, with this process as its owner.
func TestListenersFindsOwnSocket(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	port := ln.Addr().(*net.TCPAddr).Port
	all, err := Listeners()
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range all {
		if l.Port == port {
			if l.PID != os.Getpid() {
				t.Errorf("owner %d, expected this process %d", l.PID, os.Getpid())
			}
			return
		}
	}
	t.Errorf("port %d not among %d listeners", port, len(all))
}

func TestOwnMemoryAndCmdline(t *testing.T) {
	rss, err := residentMemory(os.Getpid())
	if err != nil || rss == 0 {
		t.Errorf("rss %d, %v", rss, err)
	}
	args, err := Cmdline(os.Getpid())
	if err != nil || len(args) == 0 || !sameProgram(args[0], os.Args[0]) {
		t.Errorf("cmdline %q, %v; expected to start with %q", args, err, os.Args[0])
	}
}

func TestParentPID(t *testing.T) {
	parent, err := ParentPID(os.Getpid())
	if err != nil || parent != os.Getppid() {
		t.Errorf("parent %d, %v; expected %d", parent, err, os.Getppid())
	}
}

func TestHostMemory(t *testing.T) {
	r, err := Source{}.Read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	dram := r.Pools[0]
	if dram.CapacityBytes == nil || *dram.CapacityBytes == 0 || *dram.UsedBytes > *dram.CapacityBytes {
		t.Errorf("host memory %+v; expected a size and no more than it in use", dram)
	}
}

// sameProgram compares two names of one executable, as Windows may give it
// with or without its directory and ".exe".
func sameProgram(a, b string) bool {
	name := func(p string) string { return strings.TrimSuffix(strings.ToLower(filepath.Base(p)), ".exe") }
	return name(a) == name(b)
}
