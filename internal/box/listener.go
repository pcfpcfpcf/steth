package box

import (
	"net"
	"strconv"
)

// Listener is a TCP socket listening on this machine.
type Listener struct {
	IP   net.IP
	Port int
	// PID owns the socket; 0 where this user may not see which process does.
	PID   int
	inode uint64
}

// URL is where a client on this machine reaches the listener: a socket on
// every address is reached on loopback.
func (l Listener) URL() string {
	ip := l.IP
	if ip.IsUnspecified() {
		ip = net.IPv4(127, 0, 0, 1)
		if l.IP.To4() == nil {
			ip = net.IPv6loopback
		}
	}
	return "http://" + net.JoinHostPort(ip.String(), strconv.Itoa(l.Port))
}
