//go:build !linux && !darwin

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
func parentsOfAll() (map[int]int, error)  { return nil, errUnsupported }
func ParentPID(int) (int, error)          { return 0, errUnsupported }
