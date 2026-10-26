package box

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// tcpListen is the state /proc/net/tcp gives a listening socket.
const tcpListen = "0A"

func hostMemory() (uint64, error) {
	text, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0, err
	}
	return parseKibibytes(text, "MemTotal")
}

func residentMemory(pid int) (uint64, error) {
	text, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", pid))
	if err != nil {
		return 0, err
	}
	return parseKibibytes(text, "VmRSS")
}

// Cmdline is the command line process `pid` was started with.
func Cmdline(pid int) ([]string, error) {
	raw, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
	if err != nil {
		return nil, err
	}
	return strings.Split(strings.TrimRight(string(raw), "\x00"), "\x00"), nil
}

// Listeners is every TCP socket listening on this machine, with the process
// that owns it where this user may see it (0 otherwise: another user's
// process, such as a system service).
func Listeners() ([]Listener, error) {
	var all []Listener
	for _, file := range []string{"/proc/net/tcp", "/proc/net/tcp6"} {
		text, err := os.ReadFile(file)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		found, err := parseListeners(text)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", file, err)
		}
		all = append(all, found...)
	}
	owners := socketOwners()
	for i := range all {
		all[i].PID = owners[all[i].inode]
	}
	return all, nil
}

// parseListeners reads the listening sockets of a /proc/net/tcp{,6} table.
func parseListeners(text []byte) ([]Listener, error) {
	var found []Listener
	lines := bufio.NewScanner(bytes.NewReader(text))
	lines.Scan() // the header
	for lines.Scan() {
		fields := strings.Fields(lines.Text())
		if len(fields) < 10 || fields[3] != tcpListen {
			continue
		}
		hexIP, hexPort, ok := strings.Cut(fields[1], ":")
		if !ok {
			return nil, fmt.Errorf("expected address:port, got %q", fields[1])
		}
		ip, err := parseHexIP(hexIP)
		if err != nil {
			return nil, err
		}
		port, err := strconv.ParseUint(hexPort, 16, 16)
		if err != nil {
			return nil, fmt.Errorf("expected a hex port, got %q", hexPort)
		}
		inode, err := strconv.ParseUint(fields[9], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("expected an inode, got %q", fields[9])
		}
		found = append(found, Listener{IP: ip, Port: int(port), inode: inode})
	}
	return found, lines.Err()
}

// parseHexIP reads the kernel's hex address: an IPv4 address as one
// little-endian word, an IPv6 one as four.
func parseHexIP(text string) (net.IP, error) {
	raw, err := hex.DecodeString(text)
	if err != nil || (len(raw) != net.IPv4len && len(raw) != net.IPv6len) {
		return nil, fmt.Errorf("expected a hex address, got %q", text)
	}
	for word := 0; word < len(raw); word += 4 {
		raw[word], raw[word+1], raw[word+2], raw[word+3] = raw[word+3], raw[word+2], raw[word+1], raw[word]
	}
	return net.IP(raw), nil
}

// socketOwners maps each socket inode to the process holding it, over the
// processes whose descriptors this user may read; the rest are skipped.
func socketOwners() map[uint64]int {
	owners := map[uint64]int{}
	fds, _ := filepath.Glob("/proc/[0-9]*/fd/*")
	for _, fd := range fds {
		target, err := os.Readlink(fd)
		if err != nil || !strings.HasPrefix(target, "socket:[") {
			continue
		}
		inode, err := strconv.ParseUint(strings.TrimSuffix(strings.TrimPrefix(target, "socket:["), "]"), 10, 64)
		if err != nil {
			continue
		}
		pid, _ := strconv.Atoi(strings.Split(fd, "/")[2])
		owners[inode] = pid
	}
	return owners
}
