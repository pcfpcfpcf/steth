package box

import (
	"os"
	"testing"
)

func read(t *testing.T, file string) []byte {
	text, err := os.ReadFile("testdata/" + file)
	if err != nil {
		t.Fatal(err)
	}
	return text
}

func TestGPUTotalSumsEveryGPU(t *testing.T) {
	for file, want := range map[string]uint64{"gpus.txt": 4096 << 20, "two-gpus.txt": 2 * 24576 << 20} {
		got, err := parseGPUTotal(read(t, file))
		if err != nil || got != want {
			t.Errorf("%s: %d, %v; expected %d", file, got, err, want)
		}
	}
}

func TestComputeAppsByPID(t *testing.T) {
	byPID, err := parseComputeApps(read(t, "apps.txt"))
	if err != nil || byPID[72931] != 853<<20 || byPID[72851] != 3120<<20 || len(byPID) != 2 {
		t.Errorf("got %v, %v", byPID, err)
	}
	if _, err := parseComputeApps([]byte("72931, [N/A]\n")); err == nil {
		t.Error("expected an error for memory nvidia-smi could not read")
	}
}

func TestMemTotal(t *testing.T) {
	got, err := parseKibibytes(read(t, "meminfo.txt"), "MemTotal")
	if err != nil || got != 32712788<<10 {
		t.Errorf("got %d, %v", got, err)
	}
	if _, err := parseKibibytes(read(t, "meminfo.txt"), "VmRSS"); err == nil {
		t.Error("expected an error for a field not there")
	}
}
