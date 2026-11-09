package llamacpp

import (
	"regexp"
	"strings"

	"github.com/pcfpcfpcf/steth/internal/installed"
)

// buildTag is how llama.cpp names a build: `llama --version` prints
// "b10217-ddd4ec142"; `llama-server --version` and `llama serve
// --version` print "version: 10217 (ddd4ec142)".
var (
	buildTag  = regexp.MustCompile(`^b(\d+)(-[0-9a-f]+)?$`)
	buildLine = regexp.MustCompile(`(?m)^version: (\d+) \([0-9a-f]+\)`)
)

// Binaries are the programs that serve llama.cpp, the newer first. `llama`
// is a common name, so it counts only when it says a llama.cpp build;
// llama-server is llama.cpp's own name. Each serves the one model it is
// started with: with none, the server is a router instead.
var Binaries = []installed.Binary{
	{Name: "llama", VersionArgs: []string{"--version"}, Identify: llamaBuild,
		Serve: []string{"serve", "-m", "{model}", "--port", "{port}"}, Port: Ports[0]},
	{Name: "llama-server", VersionArgs: []string{"--version"}, Identify: serverBuild,
		Serve: []string{"-m", "{model}", "--port", "{port}"}, Port: Ports[0]},
}

// llamaBuild is the build a program called llama says, and whether it
// says one: only llama.cpp's does.
func llamaBuild(output string) (string, bool) {
	build := buildOf(output)
	return build, build != ""
}

// serverBuild is llama-server's build, where it says one.
func serverBuild(output string) (string, bool) { return buildOf(output), true }

func buildOf(output string) string {
	first, _, _ := strings.Cut(strings.TrimSpace(output), "\n")
	if m := buildTag.FindStringSubmatch(strings.TrimSpace(first)); m != nil {
		return "b" + m[1]
	}
	if m := buildLine.FindStringSubmatch(output); m != nil {
		return "b" + m[1]
	}
	return ""
}
