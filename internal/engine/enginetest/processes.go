package enginetest

import (
	"context"
	"errors"
	"strings"
)

// Process is a fake server process: its pid, command line and memory.
type Process struct {
	PID  int
	Args string
	Held map[string]uint64
}

// Processes is a fake process table, by the port each server listens on; it
// stands in for local.Processes.
type Processes map[int]Process

func (p Processes) PIDAt(port int) int { return p[port].PID }

func (p Processes) Cmdline(pid int) ([]string, error) {
	proc, err := p.find(pid)
	return strings.Fields(proc.Args), err
}

func (p Processes) Memory(_ context.Context, pid int) (map[string]uint64, error) {
	proc, err := p.find(pid)
	return proc.Held, err
}

func (p Processes) find(pid int) (Process, error) {
	for _, proc := range p {
		if proc.PID == pid {
			return proc, nil
		}
	}
	return Process{}, errors.New("no such process")
}
