package box

// Parsers for what Windows prints. They carry no build tag, so they are
// tested on any system; proc_windows.go runs the commands.

import (
	"bufio"
	"bytes"
	"fmt"
	"net"
	"strconv"
	"strings"
)

// parseNetstat reads the listening TCP sockets of `netstat -ano`. A
// listening socket is known by its foreign address, which is all zeros:
// the state column is translated on Windows in other languages.
func parseNetstat(out []byte) ([]Listener, error) {
	var found []Listener
	lines := bufio.NewScanner(bytes.NewReader(out))
	for lines.Scan() {
		fields := strings.Fields(lines.Text())
		if len(fields) != 5 || (fields[0] != "TCP" && fields[0] != "TCPv6") || (fields[2] != "0.0.0.0:0" && fields[2] != "[::]:0") {
			continue
		}
		at := strings.LastIndex(fields[1], ":")
		ip := net.ParseIP(strings.Trim(fields[1][:max(at, 0)], "[]"))
		port, errPort := strconv.Atoi(fields[1][at+1:])
		pid, errPID := strconv.Atoi(fields[4])
		if at < 0 || ip == nil || errPort != nil || errPID != nil {
			return nil, fmt.Errorf("netstat: expected address:port and a pid, got %q", lines.Text())
		}
		found = append(found, Listener{IP: ip, Port: port, PID: pid})
	}
	return found, lines.Err()
}

// splitWindowsArgs splits a command line as Windows programs split theirs
// (CommandLineToArgvW): spaces separate arguments outside quotes, and
// backslashes escape a quote only when they come right before one.
func splitWindowsArgs(line string) []string {
	var args []string
	var arg strings.Builder
	quoted, started, slashes := false, false, 0
	for _, r := range line {
		switch {
		case r == '\\':
			slashes++
			started = true
			continue
		case r == '"':
			arg.WriteString(strings.Repeat(`\`, slashes/2))
			if slashes%2 == 1 {
				arg.WriteRune('"')
			} else {
				quoted = !quoted
			}
			started = true
		case (r == ' ' || r == '\t') && !quoted:
			arg.WriteString(strings.Repeat(`\`, slashes))
			if started {
				args = append(args, arg.String())
			}
			arg.Reset()
			started = false
		default:
			arg.WriteString(strings.Repeat(`\`, slashes))
			arg.WriteRune(r)
			started = true
		}
		slashes = 0
	}
	arg.WriteString(strings.Repeat(`\`, slashes))
	if started {
		args = append(args, arg.String())
	}
	return args
}
