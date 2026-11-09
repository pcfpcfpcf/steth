package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/pcfpcfpcf/steth/internal/engine/geode"
	"github.com/pcfpcfpcf/steth/internal/engine/llamacpp"
	"github.com/pcfpcfpcf/steth/internal/engine/ollama"
	"github.com/pcfpcfpcf/steth/internal/engine/vllm"
	"github.com/pcfpcfpcf/steth/internal/installed"
	"github.com/pcfpcfpcf/steth/internal/launch"
	"github.com/pcfpcfpcf/steth/internal/web"
)

// programs is every engine's programs, as its adapter names them.
var programs = []installed.Engine{
	{Name: geode.Engine, Binaries: geode.Binaries},
	{Name: ollama.Engine, Binaries: ollama.Binaries},
	{Name: llamacpp.Engine, Binaries: llamacpp.Binaries},
	{Name: vllm.Engine, Binaries: vllm.Binaries},
}

// namedPrograms collects repeated --engine ENGINE=PATH flags.
type namedPrograms []installed.Named

func (n *namedPrograms) String() string { return "" }

func (n *namedPrograms) Set(value string) error {
	name, path, found := strings.Cut(value, "=")
	if !found || name == "" || path == "" {
		return fmt.Errorf("expected ENGINE=PATH, such as geode=/opt/geode/bin/geode")
	}
	if slices.ContainsFunc(*n, func(x installed.Named) bool { return x.Engine == name }) {
		return fmt.Errorf("%s is named twice; name one program for it", name)
	}
	*n = append(*n, installed.Named{Engine: name, Path: path})
	return nil
}

// findPrograms finds each engine's program: the one named with --engine,
// else the one a configured profile runs, else the first on PATH.
func findPrograms(ctx context.Context, named []installed.Named, configured []launch.Profile) ([]installed.Found, error) {
	for _, p := range configured {
		at := slices.IndexFunc(programs, func(e installed.Engine) bool { return e.Runs(p.Command[0]) })
		if at < 0 || slices.ContainsFunc(named, func(n installed.Named) bool { return n.Engine == programs[at].Name }) {
			continue
		}
		// A profile's program not found here fails at its start, and says so.
		if path, err := exec.LookPath(p.Command[0]); err == nil {
			if path, err = filepath.Abs(path); err == nil {
				named = append(named, installed.Named{Engine: programs[at].Name, Path: path})
			}
		}
	}
	found, err := installed.Find(ctx, programs, os.Getenv("PATH"), named)
	if err != nil {
		return nil, fmt.Errorf("--engine %w", err)
	}
	return found, nil
}

// builtIn says which profiles start each engine found, and adds, for each
// one no configured profile starts, a profile of its own named after the
// engine. A configured profile that holds that name keeps it.
func builtIn(found []installed.Found, configured []launch.Profile) ([]web.Installed, []launch.Profile) {
	profiles := slices.Clone(configured)
	engines := make([]web.Installed, len(found))
	for i, f := range found {
		engines[i] = web.Installed{Found: f, Profiles: []string{}}
		at := slices.IndexFunc(programs, func(e installed.Engine) bool { return e.Name == f.Engine })
		for _, p := range configured {
			if p.Command[0] == f.Path || programs[at].Runs(p.Command[0]) {
				engines[i].Profiles = append(engines[i].Profiles, p.Name)
			}
		}
		if len(engines[i].Profiles) > 0 || f.Binary.Serve == nil {
			continue
		}
		if slices.ContainsFunc(profiles, func(p launch.Profile) bool { return p.Name == f.Engine }) {
			log.Printf("steth: no profile of steth's own for %s: a profile in the config file is named %s", f.Engine, f.Engine)
			continue
		}
		profiles = append(profiles, launch.Profile{
			Name:          f.Engine,
			Command:       append([]string{f.Path}, f.Binary.Serve...),
			Port:          f.Binary.Port,
			ModelOptional: f.Binary.ModelOptional,
		})
		engines[i].Profiles = []string{f.Engine}
	}
	return engines, profiles
}

// describe names each engine found, for the log.
func describe(found []installed.Found) string {
	if len(found) == 0 {
		return "no engine's program"
	}
	var each []string
	for _, f := range found {
		version := ""
		if f.Version != "" {
			version = " " + f.Version
		}
		each = append(each, fmt.Sprintf("%s%s at %s", f.Engine, version, f.Path))
	}
	return strings.Join(each, ", ")
}
