package box

import (
	"net"
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

func TestParseParent(t *testing.T) {
	got, err := parseParent([]byte("4242 (llama (serve) x) S 77 4242 4242 0 -1"))
	if err != nil || got != 77 {
		t.Errorf("a name with spaces and parentheses: %d, %v; expected 77", got, err)
	}
}
