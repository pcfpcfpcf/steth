//go:build unix

package launch

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"
)

// ownGroup makes the child lead a process group of its own, which the
// processes it starts join, so one signal reaches them all.
func ownGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// stopTree interrupts the child's group, kills the group after grace, and
// waits for the child's exit to be recorded.
func stopTree(child *os.Process, exited <-chan struct{}, grace time.Duration) error {
	group := -child.Pid
	if err := signalGroup(group, syscall.SIGINT); err != nil {
		return err
	}
	select {
	case <-exited:
	case <-time.After(grace):
	}
	// Killed even after the child exits, so nothing it started outlives it.
	if err := signalGroup(group, syscall.SIGKILL); err != nil {
		return err
	}
	<-exited
	return nil
}

func signalGroup(group int, signal syscall.Signal) error {
	err := syscall.Kill(group, signal)
	if err != nil && !errors.Is(err, syscall.ESRCH) {
		return fmt.Errorf("sending %v to process group %d: %w", signal, -group, err)
	}
	return nil
}
