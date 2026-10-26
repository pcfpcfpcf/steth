package box

import (
	"net"
	"testing"
)

func TestParseLsof(t *testing.T) {
	found, err := parseLsof(read(t, "lsof.txt"))
	if err != nil {
		t.Fatal(err)
	}
	want := []Listener{
		{IP: net.IPv4zero, Port: 11434, PID: 512},
		{IP: net.IPv4(127, 0, 0, 1), Port: 8080, PID: 8123},
		{IP: net.IPv6loopback, Port: 8080, PID: 8123},
	}
	if len(found) != len(want) {
		t.Fatalf("found %+v; expected %+v (the repeated socket once)", found, want)
	}
	for i := range want {
		if !found[i].IP.Equal(want[i].IP) || found[i].Port != want[i].Port || found[i].PID != want[i].PID {
			t.Errorf("listener %d: %+v, expected %+v", i, found[i], want[i])
		}
	}
	if found[0].URL() != "http://127.0.0.1:11434" {
		t.Errorf("a socket on every address reached at %s", found[0].URL())
	}
}

func TestParseVMStat(t *testing.T) {
	used, err := parseVMStat(read(t, "vm_stat.txt"))
	if want := uint64(300000+150000+100000) * 16384; err != nil || used != want {
		t.Errorf("used %d, %v; expected active + wired + compressor pages × 16 KiB = %d", used, err, want)
	}
}

func TestParsePairs(t *testing.T) {
	pairs, err := parsePairs([]byte("    1     0\n  512     1\n 8123   512\n"))
	if err != nil || pairs[8123] != 512 || len(pairs) != 3 {
		t.Errorf("pairs %v, %v", pairs, err)
	}
	if _, err := parsePairs([]byte("1 0 9\n")); err == nil {
		t.Error("expected an error for a line of three")
	}
}
