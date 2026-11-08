package launch

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"time"
)

// ownGroup does nothing on Windows: stopTree finds the child's processes
// by parentage instead.
func ownGroup(*exec.Cmd) {}

// stopTree kills the child and every process it started, and waits for the
// child's exit to be recorded. Windows has no interrupt one process can
// send another's console group without sharing it, so there is no grace
// period; taskkill is how Windows ends a tree without job objects.
func stopTree(child *os.Process, exited <-chan struct{}, _ time.Duration) error {
	tree := exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(child.Pid))
	if out, treeErr := tree.CombinedOutput(); treeErr != nil {
		err := child.Kill()
		if errors.Is(err, os.ErrProcessDone) {
			<-exited
			return nil
		}
		if err != nil {
			return fmt.Errorf("taskkill failed (%v: %s) and so did killing pid %d: %w", treeErr, out, child.Pid, err)
		}
		<-exited
		return fmt.Errorf("killed pid %d alone; what it started may still run: taskkill failed (%v: %s)", child.Pid, treeErr, out)
	}
	<-exited
	return nil
}
