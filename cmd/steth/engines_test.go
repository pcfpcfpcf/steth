package main

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/pcfpcfpcf/steth/internal/engine/geode"
	"github.com/pcfpcfpcf/steth/internal/engine/llamacpp"
	"github.com/pcfpcfpcf/steth/internal/engine/ollama"
	"github.com/pcfpcfpcf/steth/internal/engine/vllm"
	"github.com/pcfpcfpcf/steth/internal/installed"
	"github.com/pcfpcfpcf/steth/internal/launch"
)

func TestBuiltInProfileForEachEngineNotConfigured(t *testing.T) {
	found := []installed.Found{
		{Engine: "geode", Path: "/opt/geode/bin/geode", Binary: geode.Binaries[0]},
		{Engine: "ollama", Path: "/usr/local/bin/ollama", Binary: ollama.Binaries[0]},
		{Engine: "llamacpp", Path: "/home/me/.local/bin/llama", Binary: llamacpp.Binaries[0]},
		{Engine: "vllm", Path: "/venv/bin/vllm", Binary: vllm.Binaries[0]},
	}
	configured := []launch.Profile{
		{Name: "my geode", Command: []string{"/opt/geode/bin/geode", "serve", "--port", "8090"}},
		{Name: "router", Command: []string{"llama", "serve", "--models-dir", "/models"}},
		// A profile that holds an engine's name keeps it.
		{Name: "vllm", Command: []string{"/usr/bin/env", "true"}},
	}
	engines, profiles := builtIn(found, configured)

	byEngine := map[string][]string{}
	for _, e := range engines {
		byEngine[e.Engine] = e.Profiles
	}
	wantProfiles := map[string][]string{"geode": {"my geode"}, "ollama": {"ollama"}, "llamacpp": {"router"}, "vllm": {}}
	if !reflect.DeepEqual(byEngine, wantProfiles) {
		t.Errorf("profiles of each engine %v; expected %v", byEngine, wantProfiles)
	}
	want := append(slices.Clone(configured),
		launch.Profile{Name: "ollama", Command: []string{"/usr/local/bin/ollama", "serve"}, Port: 11434})
	if !reflect.DeepEqual(profiles, want) {
		t.Errorf("got %+v\nexpected %+v", profiles, want)
	}

	_, profiles = builtIn(found, nil)
	for _, c := range []struct {
		name     string
		command  []string
		port     int
		optional bool
	}{
		{"geode", []string{"/opt/geode/bin/geode", "serve", "{model}", "--models-dir", "{place}", "--port", "{port}"}, 8080, true},
		{"ollama", []string{"/usr/local/bin/ollama", "serve"}, 11434, false},
		{"llamacpp", []string{"/home/me/.local/bin/llama", "serve", "-m", "{model}", "--port", "{port}"}, 9931, false},
		{"vllm", []string{"/venv/bin/vllm", "serve", "{model}", "--port", "{port}"}, 8000, false},
	} {
		at := slices.IndexFunc(profiles, func(p launch.Profile) bool { return p.Name == c.name })
		if at < 0 {
			t.Errorf("no built-in profile for %s", c.name)
			continue
		}
		p := profiles[at]
		if !reflect.DeepEqual(p.Command, c.command) || p.Port != c.port || p.ModelOptional != c.optional {
			t.Errorf("%s: got %+v; expected command %q, port %d, model optional %v", c.name, p, c.command, c.port, c.optional)
		}
	}
}

// geode's built-in profile, found on PATH, starts with no model on a quiet
// port, allowed the places.
func TestBuiltInGeodeStartsEmpty(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fake geode is a shell script")
	}
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "geode"), []byte("#!/bin/sh\necho \"$@\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	found, err := findPrograms(context.Background(), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	engines, profiles := builtIn(found, nil)
	if len(engines) != 1 || engines[0].Engine != "geode" || !reflect.DeepEqual(engines[0].Profiles, []string{"geode"}) {
		t.Fatalf("engines %+v; expected geode alone, started by its own profile", engines)
	}
	places := t.TempDir()
	l := launch.New(profiles, func() []string { return []string{places} })
	defer l.StopAll()
	if err := l.Start("geode", ""); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		s := l.Status()[0]
		if want := "serve --models-dir " + places + " --port " + strconv.Itoa(s.Port); slices.Contains(s.Log, want) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("log %q; expected the command with no model, the place and a port from 8080", s.Log)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// A configured profile's program names its engine's program.
func TestConfiguredProgramIsFound(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fake geode is a shell script")
	}
	bin := t.TempDir()
	geodeAt := filepath.Join(bin, "geode")
	if err := os.WriteFile(geodeAt, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", t.TempDir())
	found, err := findPrograms(context.Background(), nil, []launch.Profile{{Name: "g", Command: []string{geodeAt, "serve"}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 1 || found[0].Path != geodeAt || !found[0].Named {
		t.Errorf("found %+v; expected the profile's geode", found)
	}
	if _, err := findPrograms(context.Background(), []installed.Named{{Engine: "geode", Path: "geode"}}, nil); err == nil ||
		!strings.Contains(err.Error(), "--engine geode=geode: give the program's absolute path") {
		t.Errorf("a relative --engine path: got %v", err)
	}
}
