// Package box reads what the operating system says of this machine, apart
// from any engine: how big each memory pool is, what a process holds in
// each, and which processes listen on which local ports.
package box

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/pcfpcfpcf/steth/internal/state"
)

const (
	nvidiaSMI = "nvidia-smi"
	mebibyte  = 1 << 20
	kibibyte  = 1 << 10
)

// Engine and Endpoint name the box when it is read as a source.
const (
	Engine   = "box"
	Endpoint = "local"
)

// Source reads the pools' sizes, as any engine source is read.
type Source struct{}

func (Source) Engine() string   { return Engine }
func (Source) Endpoint() string { return Endpoint }

// Read reports host memory always and GPU memory where nvidia-smi answers.
// A box without nvidia-smi reports no GPU pool rather than failing: it may
// have none, or one steth cannot read yet.
func (Source) Read(ctx context.Context) (state.Reading, error) {
	dram, dramUsed, err := hostMemory()
	if err != nil {
		return state.Reading{}, err
	}
	reading := state.Reading{Pools: []state.Pool{{Name: state.DRAM, CapacityBytes: &dram, UsedBytes: &dramUsed}}}
	gpus, err := runNvidia(ctx, "--query-gpu=memory.total,memory.used")
	if errors.Is(err, exec.ErrNotFound) {
		return reading, nil
	}
	if err != nil {
		return state.Reading{}, err
	}
	vram, vramUsed, err := parseGPUMemory(gpus)
	if err != nil {
		return state.Reading{}, err
	}
	reading.Pools = append(reading.Pools, state.Pool{Name: state.VRAM, CapacityBytes: &vram, UsedBytes: &vramUsed})
	return reading, nil
}

// ProcessMemory is what process `pid` holds in each pool: GPU memory as
// nvidia-smi reports it, and its resident host memory, which counts the
// pages of a memory-mapped model file it has touched.
func ProcessMemory(ctx context.Context, pid int) (map[string]uint64, error) {
	held := map[string]uint64{}
	rss, err := residentMemory(pid)
	if err != nil {
		return nil, err
	}
	held[state.DRAM] = rss
	apps, err := runNvidia(ctx, "--query-compute-apps=pid,used_memory")
	if errors.Is(err, exec.ErrNotFound) {
		return held, nil
	}
	if err != nil {
		return nil, err
	}
	byPID, err := parseComputeApps(apps)
	if err != nil {
		return nil, err
	}
	held[state.VRAM] = byPID[pid]
	return held, nil
}

func runNvidia(ctx context.Context, query string) ([]byte, error) {
	out, err := exec.CommandContext(ctx, nvidiaSMI, query, "--format=csv,noheader,nounits").Output()
	if err != nil {
		return nil, fmt.Errorf("%s %s: %w", nvidiaSMI, query, err)
	}
	return out, nil
}

// csvRows splits nvidia-smi's "a, b" lines into fields.
func csvRows(out []byte) [][]string {
	var rows [][]string
	lines := bufio.NewScanner(bytes.NewReader(out))
	for lines.Scan() {
		line := strings.TrimSpace(lines.Text())
		if line == "" {
			continue
		}
		fields := strings.Split(line, ",")
		for i := range fields {
			fields[i] = strings.TrimSpace(fields[i])
		}
		rows = append(rows, fields)
	}
	return rows
}

// parseGPUMemory sums every GPU's memory and what is in use of it: steth
// draws them as one pool.
func parseGPUMemory(out []byte) (total, used uint64, err error) {
	for _, row := range csvRows(out) {
		if len(row) < 2 {
			return 0, 0, fmt.Errorf("nvidia-smi: expected memory.total, memory.used; got %q", strings.Join(row, ","))
		}
		for i, sum := range []*uint64{&total, &used} {
			mib, err := strconv.ParseUint(row[i], 10, 64)
			if err != nil {
				return 0, 0, fmt.Errorf("nvidia-smi: expected MiB, got %q", row[i])
			}
			*sum += mib * mebibyte
		}
	}
	return total, used, nil
}

// parseComputeApps maps each process to the GPU memory it holds, summed
// over GPUs.
func parseComputeApps(out []byte) (map[int]uint64, error) {
	byPID := map[int]uint64{}
	for _, row := range csvRows(out) {
		if len(row) < 2 {
			return nil, fmt.Errorf("nvidia-smi compute apps: expected pid, used_memory; got %q", strings.Join(row, ","))
		}
		pid, err := strconv.Atoi(row[0])
		if err != nil {
			return nil, fmt.Errorf("nvidia-smi compute apps: expected a pid, got %q", row[0])
		}
		mib, err := strconv.ParseUint(row[1], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("nvidia-smi compute apps: expected MiB, got %q", row[1])
		}
		byPID[pid] += mib * mebibyte
	}
	return byPID, nil
}

// parseKibibytes reads the "Name: N kB" field of a /proc file.
func parseKibibytes(text []byte, name string) (uint64, error) {
	lines := bufio.NewScanner(bytes.NewReader(text))
	for lines.Scan() {
		key, value, found := strings.Cut(lines.Text(), ":")
		if !found || key != name {
			continue
		}
		kib, err := strconv.ParseUint(strings.TrimSuffix(strings.TrimSpace(value), " kB"), 10, 64)
		if err != nil {
			return 0, fmt.Errorf("%s: expected kB, got %q", name, value)
		}
		return kib * kibibyte, nil
	}
	return 0, fmt.Errorf("%s: not found", name)
}
