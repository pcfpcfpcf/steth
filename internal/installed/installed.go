// Package installed finds the engines whose programs are on this machine:
// named by the person at the box, or on PATH. It never invents one: an
// engine whose program is not found is absent.
package installed

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"
)

const (
	// A program asked its version answers at once or not at all.
	versionTimeout = 2 * time.Second
	// versionBytes is how much of its answer is read.
	versionBytes = 4 << 10
	// pipeGrace is how long its output is still read after it exits, while
	// something it started holds the pipe open.
	pipeGrace = 500 * time.Millisecond
)

// Binary is one program that serves an engine, as the engine's adapter
// describes it.
type Binary struct {
	// Name is its file name, without .exe.
	Name string
	// VersionArgs ask it for its version; nil runs nothing, for a program
	// whose version is slow to ask or that has no such flag.
	VersionArgs []string
	// Identify reads what VersionArgs printed, empty if it printed nothing:
	// the version, and whether the program is this engine's at all. Nil
	// means the name is enough.
	Identify func(output string) (version string, ours bool)
	// Serve is the arguments after the program that start the engine, with
	// launch's placeholders; nil for none.
	Serve []string
	// ModelOptional says Serve also starts with no model.
	ModelOptional bool
	// Port is where Serve listens: the first port its {port} tries, or,
	// without {port}, the port it always takes.
	Port int
}

// Engine is an engine's name and the programs that serve it, preferred
// first.
type Engine struct {
	Name     string
	Binaries []Binary
}

// Named is a program the person at the box says serves an engine.
type Named struct {
	Engine string
	Path   string
}

// Found is an engine's program on this machine.
type Found struct {
	Engine  string `json:"engine"`
	Path    string `json:"path"`
	Version string `json:"version,omitempty"`
	// Named says it was named, not found on PATH.
	Named  bool   `json:"named"`
	Binary Binary `json:"-"`
}

// Find lists the engines whose program is here, in the order of engines:
// the one named for each, else the first of its binaries on path (a PATH
// list) that identifies as its own. A named program is trusted; it is only
// asked its version.
func Find(ctx context.Context, engines []Engine, path string, named []Named) ([]Found, error) {
	var found []Found
	for _, n := range named {
		if !slices.ContainsFunc(engines, func(e Engine) bool { return e.Name == n.Engine }) {
			return nil, fmt.Errorf("%s=%s: no engine is named %s; name one of %s", n.Engine, n.Path, n.Engine, names(engines))
		}
	}
	for _, e := range engines {
		if at := slices.IndexFunc(named, func(n Named) bool { return n.Engine == e.Name }); at >= 0 {
			f, err := trust(ctx, e, named[at].Path)
			if err != nil {
				return nil, err
			}
			found = append(found, f)
			continue
		}
		if f, ok := look(ctx, e, path); ok {
			found = append(found, f)
		}
	}
	return found, nil
}

// trust is a named program, refused only when it is not one.
func trust(ctx context.Context, e Engine, path string) (Found, error) {
	if !filepath.IsAbs(path) {
		return Found{}, fmt.Errorf("%s=%s: give the program's absolute path", e.Name, path)
	}
	if _, err := exec.LookPath(path); err != nil {
		return Found{}, fmt.Errorf("%s=%s: %w; give the path of an executable file", e.Name, path, err)
	}
	b := e.Binaries[0]
	if at := slices.IndexFunc(e.Binaries, func(b Binary) bool { return b.Name == programName(path) }); at >= 0 {
		b = e.Binaries[at]
	}
	version, _ := identify(ctx, b, path)
	return Found{Engine: e.Name, Path: path, Version: version, Named: true, Binary: b}, nil
}

// look finds e's first binary on path that is e's.
func look(ctx context.Context, e Engine, path string) (Found, bool) {
	for _, b := range e.Binaries {
		program, ok := onPath(b.Name, path)
		if !ok {
			continue
		}
		if version, ours := identify(ctx, b, program); ours {
			return Found{Engine: e.Name, Path: program, Version: version, Binary: b}, true
		}
	}
	return Found{}, false
}

// onPath is the first executable file called name in a directory of path.
// A relative directory is skipped, as the shell's current one would be.
func onPath(name, path string) (string, bool) {
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	for _, dir := range filepath.SplitList(path) {
		if !filepath.IsAbs(dir) {
			continue
		}
		candidate := filepath.Join(dir, name)
		if _, err := exec.LookPath(candidate); err == nil {
			return candidate, true
		}
	}
	return "", false
}

// identify asks program its version, if b says how, and whether it is b's.
func identify(ctx context.Context, b Binary, program string) (string, bool) {
	if b.Identify == nil {
		return "", true
	}
	var output string
	if b.VersionArgs != nil {
		output = ask(ctx, program, b.VersionArgs)
	}
	return b.Identify(output)
}

// ask runs program with args and returns the start of what it printed,
// whether or not it succeeded: a failure says it is not the program
// expected through what it printed, or did not print.
func ask(ctx context.Context, program string, args []string) string {
	ctx, cancel := context.WithTimeout(ctx, versionTimeout)
	defer cancel()
	out := &capped{limit: versionBytes}
	cmd := exec.CommandContext(ctx, program, args...)
	cmd.Stdout, cmd.Stderr = out, out
	cmd.WaitDelay = pipeGrace
	cmd.Run() // its failure is in what it printed, or that it printed nothing
	return out.String()
}

// capped keeps the first limit bytes written to it.
type capped struct {
	strings.Builder
	limit int
}

func (c *capped) Write(p []byte) (int, error) {
	if room := c.limit - c.Len(); room > 0 {
		c.Builder.Write(p[:min(len(p), room)])
	}
	return len(p), nil
}

// Runs says whether program, a command's first argument, is one of e's
// binaries, by its file name.
func (e Engine) Runs(program string) bool {
	return slices.ContainsFunc(e.Binaries, func(b Binary) bool { return b.Name == programName(program) })
}

// programName is a program's file name without .exe.
func programName(program string) string {
	base := filepath.Base(program)
	if ext := filepath.Ext(base); strings.EqualFold(ext, ".exe") {
		base = strings.TrimSuffix(base, ext)
	}
	return base
}

func names(engines []Engine) string {
	all := make([]string, len(engines))
	for i, e := range engines {
		all[i] = e.Name
	}
	return strings.Join(all, ", ")
}
