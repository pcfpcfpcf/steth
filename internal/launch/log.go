package launch

import (
	"bytes"
	"strings"
	"sync"
	"unicode/utf8"
)

const (
	maxLogLines  = 120
	maxLineChars = 2000
	// maxLineBytes bounds a line still being written: enough for
	// maxLineChars characters of any width, and one byte to show it ran over.
	maxLineBytes = maxLineChars*utf8.UTFMax + 1
	// cutMark ends a line that was cut; it counts as one of its characters.
	cutMark = "…"
)

// lineLog keeps the last maxLogLines lines a child wrote, each cut to
// maxLineChars characters. It does no locking of its own.
type lineLog struct {
	kept    []string
	partial []byte // the line being written, at most maxLineBytes
}

func (g *lineLog) write(p []byte) {
	for {
		end := bytes.IndexByte(p, '\n')
		if end < 0 {
			g.extend(p)
			return
		}
		g.extend(p[:end])
		g.endLine()
		p = p[end+1:]
	}
}

// finish keeps a last line the child wrote without a newline.
func (g *lineLog) finish() {
	if len(g.partial) > 0 {
		g.endLine()
	}
}

func (g *lineLog) lines() []string {
	return append([]string{}, g.kept...)
}

func (g *lineLog) extend(p []byte) {
	room := maxLineBytes - len(g.partial)
	g.partial = append(g.partial, p[:min(len(p), room)]...)
}

func (g *lineLog) endLine() {
	line := cut(strings.TrimSuffix(string(g.partial), "\r"))
	g.partial = g.partial[:0]
	if len(g.kept) == maxLogLines {
		g.kept = append(g.kept[:0], g.kept[1:]...)
	}
	g.kept = append(g.kept, line)
}

func cut(line string) string {
	if utf8.RuneCountInString(line) <= maxLineChars {
		return line
	}
	return string([]rune(line)[:maxLineChars-1]) + cutMark
}

// lockedLog is a child's stdout and stderr: one writer, so the two arrive
// through one pipe in the order the child wrote them.
type lockedLog struct {
	mu  *sync.Mutex
	log *lineLog
}

func (w *lockedLog) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.log.write(p)
	return len(p), nil
}
