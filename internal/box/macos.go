package box

// Parsers for what macOS's commands print. They carry no build tag, so they
// are tested on any system; proc_darwin.go runs the commands.

import (
	"bufio"
	"bytes"
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"
)

// parseLsof reads `lsof -nP -iTCP -sTCP:LISTEN -F pn`: a "p<pid>" line, then
// an "n<address>:<port>" line per socket of that process.
func parseLsof(out []byte) ([]Listener, error) {
	var found []Listener
	seen := map[string]bool{}
	pid := 0
	lines := bufio.NewScanner(bytes.NewReader(out))
	for lines.Scan() {
		line := lines.Text()
		if line == "" {
			continue
		}
		switch line[0] {
		case 'p':
			p, err := strconv.Atoi(line[1:])
			if err != nil {
				return nil, fmt.Errorf("lsof: expected a pid, got %q", line)
			}
			pid = p
		case 'n':
			at := strings.LastIndex(line, ":")
			if at < 0 {
				return nil, fmt.Errorf("lsof: expected address:port, got %q", line)
			}
			port, err := strconv.Atoi(line[at+1:])
			if err != nil {
				return nil, fmt.Errorf("lsof: expected a port, got %q", line)
			}
			host := strings.Trim(line[1:at], "[]")
			ip := net.IPv4zero
			if host != "*" {
				if ip = net.ParseIP(host); ip == nil {
					return nil, fmt.Errorf("lsof: expected an address, got %q", host)
				}
			}
			key := fmt.Sprint(pid, ip, port)
			if !seen[key] {
				seen[key] = true
				found = append(found, Listener{IP: ip, Port: port, PID: pid})
			}
		}
	}
	return found, lines.Err()
}

var pageSize = regexp.MustCompile(`page size of (\d+) bytes`)

// parseVMStat reads `vm_stat` into memory in use: active, wired and
// compressed pages, as Activity Monitor counts "memory used".
func parseVMStat(out []byte) (uint64, error) {
	size := pageSize.FindSubmatch(out)
	if size == nil {
		return 0, fmt.Errorf("vm_stat: no page size in %q", firstLine(out))
	}
	bytesPerPage, _ := strconv.ParseUint(string(size[1]), 10, 64)
	var pages uint64
	for _, name := range []string{"Pages active", "Pages wired down", "Pages occupied by compressor"} {
		n, err := vmStatField(out, name)
		if err != nil {
			return 0, err
		}
		pages += n
	}
	return pages * bytesPerPage, nil
}

func vmStatField(out []byte, name string) (uint64, error) {
	lines := bufio.NewScanner(bytes.NewReader(out))
	for lines.Scan() {
		key, value, found := strings.Cut(lines.Text(), ":")
		if found && key == name {
			n, err := strconv.ParseUint(strings.TrimSuffix(strings.TrimSpace(value), "."), 10, 64)
			if err != nil {
				return 0, fmt.Errorf("vm_stat %s: expected pages, got %q", name, value)
			}
			return n, nil
		}
	}
	return 0, fmt.Errorf("vm_stat: no %q", name)
}

// parsePairs reads lines of two numbers, such as `ps -A -o pid=,ppid=`.
func parsePairs(out []byte) (map[int]int, error) {
	pairs := map[int]int{}
	lines := bufio.NewScanner(bytes.NewReader(out))
	for lines.Scan() {
		fields := strings.Fields(lines.Text())
		if len(fields) == 0 {
			continue
		}
		if len(fields) != 2 {
			return nil, fmt.Errorf("expected two numbers, got %q", lines.Text())
		}
		a, errA := strconv.Atoi(fields[0])
		b, errB := strconv.Atoi(fields[1])
		if errA != nil || errB != nil {
			return nil, fmt.Errorf("expected two numbers, got %q", lines.Text())
		}
		pairs[a] = b
	}
	return pairs, lines.Err()
}

func firstLine(out []byte) string {
	line, _, _ := bytes.Cut(out, []byte("\n"))
	return string(line)
}
