package ollama

import (
	"regexp"

	"github.com/pcfpcfpcf/steth/internal/installed"
)

// versionLine is how `ollama --version` names its version: "ollama version
// is 0.21.2" with a server, "client version is 0.21.2" without.
var versionLine = regexp.MustCompile(`version is (\S+)`)

// Binaries is the program that serves Ollama. `ollama serve` takes no
// port flag; it listens where OLLAMA_HOST says, by default on Ports[0].
var Binaries = []installed.Binary{{
	Name:        "ollama",
	VersionArgs: []string{"--version"},
	Identify:    version,
	Serve:       []string{"serve"},
	Port:        Ports[0],
}}

// version reads `ollama --version`; the name is enough to be Ollama's.
func version(output string) (string, bool) {
	if m := versionLine.FindStringSubmatch(output); m != nil {
		return m[1], true
	}
	return "", true
}
