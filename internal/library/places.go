// Package library is where model files are on this machine: the places
// steth looks (directories and .gguf files, from the config and added from
// the page), the model files under them, and a listing of one directory
// for picking a place.
package library

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
)

// Place is a directory or .gguf file steth looks in for model files.
type Place struct {
	Path string `json:"path"`
	// Fixed is a place from the config file, which the page cannot remove.
	Fixed bool `json:"fixed"`
}

// Places are the config's places and those added from the page, which are
// kept in a file so they outlast steth. It is safe for concurrent use.
type Places struct {
	fixed []string
	file  string // where added places are kept; "" keeps them in memory only

	mu    sync.Mutex
	added []string
}

// Open returns the config's fixed places and those added before, read
// from file when it is there. A saved place gone since is kept: a disk can
// be unmounted, and Scan skips it.
func Open(fixed []string, file string) (*Places, error) {
	p := &Places{fixed: cleanAll(fixed), file: file}
	if file == "" {
		return p, nil
	}
	text, err := os.ReadFile(file)
	if errors.Is(err, os.ErrNotExist) {
		return p, nil
	}
	if err != nil {
		return nil, err
	}
	var saved struct {
		Places []string `json:"places"`
	}
	if err := json.Unmarshal(text, &saved); err != nil {
		return nil, fmt.Errorf("%s: %w; it holds {\"places\": [...]}, written by steth", file, err)
	}
	for _, path := range cleanAll(saved.Places) {
		if filepath.IsAbs(path) && !slices.Contains(p.fixed, path) && !slices.Contains(p.added, path) {
			p.added = append(p.added, path)
		}
	}
	return p, nil
}

// All are the fixed places, then the added ones in the order added.
func (p *Places) All() []Place {
	p.mu.Lock()
	defer p.mu.Unlock()
	all := make([]Place, 0, len(p.fixed)+len(p.added))
	for _, path := range p.fixed {
		all = append(all, Place{Path: path, Fixed: true})
	}
	for _, path := range p.added {
		all = append(all, Place{Path: path})
	}
	return all
}

// Paths are All's paths.
func (p *Places) Paths() []string {
	all := p.All()
	paths := make([]string, len(all))
	for i, place := range all {
		paths[i] = place.Path
	}
	return paths
}

// Add adds path, an absolute directory or .gguf file that is there, and
// saves the added places.
func (p *Places) Add(path string) error {
	if err := Check(path); err != nil {
		return err
	}
	path = filepath.Clean(path)
	p.mu.Lock()
	defer p.mu.Unlock()
	if slices.Contains(p.fixed, path) || slices.Contains(p.added, path) {
		return fmt.Errorf("%s is a place already", path)
	}
	added := append(slices.Clip(p.added), path)
	if err := p.save(added); err != nil {
		return err
	}
	p.added = added
	return nil
}

// Remove removes an added place and saves the rest.
func (p *Places) Remove(path string) error {
	path = filepath.Clean(path)
	p.mu.Lock()
	defer p.mu.Unlock()
	if slices.Contains(p.fixed, path) {
		return fmt.Errorf("%s comes from the config file; remove it there", path)
	}
	i := slices.Index(p.added, path)
	if i < 0 {
		return fmt.Errorf("%s is not a place", path)
	}
	added := slices.Delete(slices.Clone(p.added), i, i+1)
	if err := p.save(added); err != nil {
		return err
	}
	p.added = added
	return nil
}

// Check says whether path can be a place: absolute, there, and a directory
// or a .gguf file.
func Check(path string) error {
	if !filepath.IsAbs(path) {
		return fmt.Errorf("%q is not an absolute path", path)
	}
	info, err := os.Stat(path)
	switch {
	case err != nil:
		return err
	case !info.IsDir() && !(info.Mode().IsRegular() && strings.EqualFold(filepath.Ext(path), modelExt)):
		return fmt.Errorf("%s is neither a directory nor a %s file", path, modelExt)
	}
	return nil
}

// save writes the added places to the file, through a temporary file so a
// crash leaves the old list or the new one. It runs under p.mu.
func (p *Places) save(added []string) error {
	if p.file == "" {
		return nil
	}
	text, err := json.MarshalIndent(struct {
		Places []string `json:"places"`
	}{append([]string{}, added...)}, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p.file), 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(p.file), ".places-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(append(text, '\n')); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), p.file)
}

func cleanAll(paths []string) []string {
	out := make([]string, 0, len(paths))
	for _, path := range paths {
		if path = filepath.Clean(path); !slices.Contains(out, path) {
			out = append(out, path)
		}
	}
	return out
}
