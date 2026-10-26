//go:build !linux

package box

import (
	"errors"
	"fmt"
	"runtime"
)

var errUnsupported = fmt.Errorf("reading the box on %s: %w", runtime.GOOS, errors.ErrUnsupported)

func hostMemory() (uint64, uint64, error) { return 0, 0, errUnsupported }
func residentMemory(int) (uint64, error)  { return 0, errUnsupported }
func Cmdline(int) ([]string, error)       { return nil, errUnsupported }
func Listeners() ([]Listener, error)      { return nil, errUnsupported }
