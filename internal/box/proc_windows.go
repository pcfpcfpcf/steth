package box

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

// A command the box runs gets this long; PowerShell takes most of a second
// to start.
const commandTimeout = 5 * time.Second

// processQueryLimited is PROCESS_QUERY_LIMITED_INFORMATION: enough to read a
// process's memory, granted for other users' processes too.
const processQueryLimited = 0x1000

var (
	kernel32             = syscall.NewLazyDLL("kernel32.dll")
	globalMemoryStatusEx = kernel32.NewProc("GlobalMemoryStatusEx")
	getProcessMemoryInfo = kernel32.NewProc("K32GetProcessMemoryInfo")
)

// memoryStatusEx is MEMORYSTATUSEX.
type memoryStatusEx struct {
	length               uint32
	memoryLoad           uint32
	totalPhys            uint64
	availPhys            uint64
	totalPageFile        uint64
	availPageFile        uint64
	totalVirtual         uint64
	availVirtual         uint64
	availExtendedVirtual uint64
}

// processMemoryCounters is PROCESS_MEMORY_COUNTERS.
type processMemoryCounters struct {
	cb                         uint32
	pageFaultCount             uint32
	peakWorkingSetSize         uintptr
	workingSetSize             uintptr
	quotaPeakPagedPoolUsage    uintptr
	quotaPagedPoolUsage        uintptr
	quotaPeakNonPagedPoolUsage uintptr
	quotaNonPagedPoolUsage     uintptr
	pagefileUsage              uintptr
	peakPagefileUsage          uintptr
}

func hostMemory() (total, used uint64, err error) {
	status := memoryStatusEx{length: uint32(unsafe.Sizeof(memoryStatusEx{}))}
	if ok, _, err := globalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&status))); ok == 0 {
		return 0, 0, fmt.Errorf("GlobalMemoryStatusEx: %w", err)
	}
	return status.totalPhys, status.totalPhys - min(status.availPhys, status.totalPhys), nil
}

// residentMemory is the process's working set, Windows' resident memory.
func residentMemory(pid int) (uint64, error) {
	handle, err := syscall.OpenProcess(processQueryLimited, false, uint32(pid))
	if err != nil {
		return 0, fmt.Errorf("process %d: %w", pid, err)
	}
	defer syscall.CloseHandle(handle)
	counters := processMemoryCounters{cb: uint32(unsafe.Sizeof(processMemoryCounters{}))}
	if ok, _, err := getProcessMemoryInfo.Call(uintptr(handle), uintptr(unsafe.Pointer(&counters)), uintptr(counters.cb)); ok == 0 {
		return 0, fmt.Errorf("process %d memory: %w", pid, err)
	}
	return uint64(counters.workingSetSize), nil
}

func parentsOfAll() (map[int]int, error) {
	snapshot, err := syscall.CreateToolhelp32Snapshot(syscall.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, fmt.Errorf("listing processes: %w", err)
	}
	defer syscall.CloseHandle(snapshot)
	parents := map[int]int{}
	entry := syscall.ProcessEntry32{Size: uint32(unsafe.Sizeof(syscall.ProcessEntry32{}))}
	for err = syscall.Process32First(snapshot, &entry); err == nil; err = syscall.Process32Next(snapshot, &entry) {
		parents[int(entry.ProcessID)] = int(entry.ParentProcessID)
	}
	return parents, nil
}

func ParentPID(pid int) (int, error) {
	parents, err := parentsOfAll()
	if err != nil {
		return 0, err
	}
	parent, ok := parents[pid]
	if !ok {
		return 0, fmt.Errorf("process %d: not running", pid)
	}
	return parent, nil
}

func command(name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, name, args...).Output()
	if err != nil {
		return nil, fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
	}
	return out, nil
}

// Cmdline asks PowerShell, since Windows keeps a process's command line
// inside the process.
func Cmdline(pid int) ([]string, error) {
	out, err := command("powershell", "-NoProfile", "-NonInteractive", "-Command",
		"(Get-CimInstance Win32_Process -Filter 'ProcessId="+strconv.Itoa(pid)+"').CommandLine")
	if err != nil {
		return nil, err
	}
	line := strings.TrimSpace(string(out))
	if line == "" {
		return nil, fmt.Errorf("process %d: no command line", pid)
	}
	return splitWindowsArgs(line), nil
}

// Listeners is every TCP socket listening, IPv4 and IPv6, each with its
// process.
func Listeners() ([]Listener, error) {
	var all []Listener
	for _, proto := range []string{"TCP", "TCPv6"} {
		out, err := command("netstat", "-ano", "-p", proto)
		if err != nil {
			return nil, err
		}
		found, err := parseNetstat(out)
		if err != nil {
			return nil, err
		}
		all = append(all, found...)
	}
	return all, nil
}
