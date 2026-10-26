package llamacpp

import (
	"slices"
	"strconv"
)

// launch is a server's command line.
type launch []string

// value is what follows the first of `names` given, as llama.cpp's own
// parser reads "--flag value".
func (l launch) value(names ...string) (string, bool) {
	for i, arg := range l[:max(len(l)-1, 0)] {
		if slices.Contains(names, arg) {
			return l[i+1], true
		}
	}
	return "", false
}

func (l launch) has(names ...string) bool {
	return slices.ContainsFunc(l, func(a string) bool { return slices.Contains(names, a) })
}

// port is where the server listens, 0 when its command line does not say.
func (l launch) port() int {
	v, _ := l.value("--port")
	port, _ := strconv.Atoi(v)
	return port
}

// sleeps is whether the server frees its model when idle.
func (l launch) sleeps() bool {
	v, given := l.value("--sleep-idle-seconds")
	seconds, err := strconv.Atoi(v)
	return given && err == nil && seconds > 0
}

// unifiedKV is whether every slot shares one KV cache: by default only when
// the number of slots is left to the server.
func (l launch) unifiedKV() bool {
	switch {
	case l.has("-kvu", "--kv-unified"):
		return true
	case l.has("-no-kvu", "--no-kv-unified"):
		return false
	}
	slots, given := l.value("-np", "--parallel")
	return !given || slots == "-1"
}
