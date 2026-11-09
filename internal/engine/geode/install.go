package geode

import "github.com/pcfpcfpcf/steth/internal/installed"

// Binaries is the program that serves geode. It is not asked a version:
// geode has no version flag and reads one as a model to open. It serves
// with no model and loads files from the places later.
var Binaries = []installed.Binary{{
	Name:          "geode",
	Serve:         []string{"serve", "{model}", "--models-dir", "{place}", "--port", "{port}"},
	ModelOptional: true,
	Port:          Ports[0],
}}
