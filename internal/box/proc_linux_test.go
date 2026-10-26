package box

import (
	"net"
	"os"
	"testing"
)

func TestParseListeners(t *testing.T) {
	found, err := parseListeners(read(t, "tcp.txt"))
	if err != nil {
		t.Fatal(err)
	}
	// 0100007F:2CAA is 127.0.0.1:11434, Ollama's port.
	var ollama *Listener
	for i := range found {
		if found[i].Port == 11434 {
			ollama = &found[i]
		}
	}
	if ollama == nil || !ollama.IP.Equal(net.IPv4(127, 0, 0, 1)) || ollama.URL() != "http://127.0.0.1:11434" {
		t.Errorf("listeners %+v; expected 127.0.0.1:11434", found)
	}
}

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
	if err != nil || len(args) == 0 || args[0] != os.Args[0] {
		t.Errorf("cmdline %q, %v; expected to start with %q", args, err, os.Args[0])
	}
}

func TestParentPID(t *testing.T) {
	parent, err := ParentPID(os.Getpid())
	if err != nil || parent != os.Getppid() {
		t.Errorf("parent %d, %v; expected %d", parent, err, os.Getppid())
	}
	got, err := parseParent([]byte("4242 (llama (serve) x) S 77 4242 4242 0 -1"))
	if err != nil || got != 77 {
		t.Errorf("a name with spaces and parentheses: %d, %v; expected 77", got, err)
	}
}
