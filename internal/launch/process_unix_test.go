//go:build unix

package launch

import (
	"errors"
	"syscall"
	"testing"
)

// watchProcess says, each time it is asked, whether pid has ended.
func watchProcess(t *testing.T, pid int) func() bool {
	return func() bool { return errors.Is(syscall.Kill(pid, 0), syscall.ESRCH) }
}
