package launch

import (
	"os"
	"testing"
)

// watchProcess says, each time it is asked, whether pid has ended. The
// handle is opened at once, so the pid cannot be reused while watched.
func watchProcess(t *testing.T, pid int) func() bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		t.Fatal(err)
	}
	ended := make(chan struct{})
	go func() {
		if _, err := process.Wait(); err != nil {
			t.Errorf("waiting for pid %d: %v", pid, err)
		}
		close(ended)
	}()
	return func() bool {
		select {
		case <-ended:
			return true
		default:
			return false
		}
	}
}
