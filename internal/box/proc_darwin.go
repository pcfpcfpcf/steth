package box

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// A command the box runs gets this long; ps, lsof and vm_stat answer in
// milliseconds.
const commandTimeout = 2 * time.Second

func command(name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, name, args...).Output()
	if err != nil {
		return nil, fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
	}
	return out, nil
}

func hostMemory() (total, used uint64, err error) {
	out, err := command("sysctl", "-n", "hw.memsize")
	if err != nil {
		return 0, 0, err
	}
	total, err = strconv.ParseUint(strings.TrimSpace(string(out)), 10, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("sysctl hw.memsize: expected bytes, got %q", out)
	}
	stat, err := command("vm_stat")
	if err != nil {
		return 0, 0, err
	}
	used, err = parseVMStat(stat)
	return total, min(used, total), err
}

// psField is one `ps -o` field of process `pid`.
func psField(pid int, field string) (string, error) {
	out, err := command("ps", "-ww", "-o", field+"=", "-p", strconv.Itoa(pid))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func residentMemory(pid int) (uint64, error) {
	v, err := psField(pid, "rss")
	if err != nil {
		return 0, err
	}
	kib, err := strconv.ParseUint(v, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("ps rss: expected KiB, got %q", v)
	}
	return kib * kibibyte, nil
}

// Cmdline is the command line process `pid` was started with, split on
// spaces: ps does not say where an argument with a space in it ends.
func Cmdline(pid int) ([]string, error) {
	v, err := psField(pid, "args")
	if err != nil {
		return nil, err
	}
	return strings.Fields(v), nil
}

func ParentPID(pid int) (int, error) {
	v, err := psField(pid, "ppid")
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(v)
}

func parentsOfAll() (map[int]int, error) {
	out, err := command("ps", "-A", "-o", "pid=,ppid=")
	if err != nil {
		return nil, err
	}
	return parsePairs(out)
}

// Listeners is every TCP socket listening that lsof shows this user: its
// own processes' and, run as root, everyone's.
func Listeners() ([]Listener, error) {
	out, err := command("lsof", "-nP", "-iTCP", "-sTCP:LISTEN", "-F", "pn")
	// lsof exits 1 when it finds nothing.
	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.ExitCode() == 1 {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return parseLsof(out)
}
