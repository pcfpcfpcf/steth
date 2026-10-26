//go:build !windows

package box

import (
	"context"
	"os"
	"os/exec"
	"slices"
	"testing"
)

// A child this test starts is in its tree, and its memory counts.
func TestProcessTree(t *testing.T) {
	child := exec.Command("sleep", "5")
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	defer child.Process.Kill()
	tree, err := processTree(os.Getpid())
	if err != nil || tree[0] != os.Getpid() || !slices.Contains(tree, child.Process.Pid) {
		t.Errorf("tree %v, %v; expected this process, then its child %d", tree, err, child.Process.Pid)
	}
	childRSS, _ := residentMemory(child.Process.Pid)
	own, _ := residentMemory(os.Getpid())
	held, err := ProcessMemory(context.Background(), os.Getpid())
	if err != nil || held["dram"] < childRSS || held["dram"] < own/2 {
		t.Errorf("tree holds %d; expected this process's ~%d and its child's %d", held["dram"], own, childRSS)
	}
}
