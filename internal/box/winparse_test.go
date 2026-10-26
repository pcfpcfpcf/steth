package box

import (
	"net"
	"slices"
	"testing"
)

func TestParseNetstat(t *testing.T) {
	found, err := parseNetstat(read(t, "netstat.txt"))
	if err != nil {
		t.Fatal(err)
	}
	want := []Listener{
		{IP: net.IPv4zero, Port: 135, PID: 1012},
		{IP: net.IPv4(127, 0, 0, 1), Port: 11434, PID: 8812},
		{IP: net.IPv6unspecified, Port: 135, PID: 1012},
		{IP: net.IPv6loopback, Port: 8080, PID: 4410},
	}
	if len(found) != len(want) {
		t.Fatalf("found %+v; expected %d listeners, connections left out", found, len(want))
	}
	for i := range want {
		if !found[i].IP.Equal(want[i].IP) || found[i].Port != want[i].Port || found[i].PID != want[i].PID {
			t.Errorf("listener %d: %+v, expected %+v", i, found[i], want[i])
		}
	}
}

func TestSplitWindowsArgs(t *testing.T) {
	for line, want := range map[string][]string{
		`"C:\Program Files\llama\llama.exe" serve -m "D:\models\a b.gguf"`: {`C:\Program Files\llama\llama.exe`, "serve", "-m", `D:\models\a b.gguf`},
		`ollama.exe  serve`:                            {"ollama.exe", "serve"},
		`a\\b "c\"d" e\\\"f ""`:                        {`a\\b`, `c"d`, `e\"f`, ""},
		`python -m vllm.entrypoints.openai.api_server`: {"python", "-m", "vllm.entrypoints.openai.api_server"},
	} {
		if got := splitWindowsArgs(line); !slices.Equal(got, want) {
			t.Errorf("%s: %q, expected %q", line, got, want)
		}
	}
}
