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

func TestGPUMemorySumsEveryGPU(t *testing.T) {
	for file, want := range map[string][2]uint64{
		"gpus.txt":     {4096 << 20, 872 << 20},
		"two-gpus.txt": {2 * 24576 << 20, (1024 + 20480) << 20},
	} {
		total, used, err := parseGPUMemory(read(t, file))
		if err != nil || total != want[0] || used != want[1] {
			t.Errorf("%s: %d, %d, %v; expected %d, %d", file, total, used, err, want[0], want[1])
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
