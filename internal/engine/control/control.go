// Package control is what an adapter offers when its engine can load and
// unload models over its own HTTP API. An adapter without the controls does
// not implement Controls, so a new engine starts read-only.
package control

import (
	"context"
	"errors"
)

// ErrNone is returned by Available when this server has no controls after
// all: a llama.cpp server of one model, a vLLM server not in dev mode.
var ErrNone = errors.New("this server has no load or unload controls")

// Controls loads and unloads models through the engine's own API.
type Controls interface {
	// Available lists the models the server could load or unload now, by
	// the id its own API takes; ErrNone when it has no controls.
	Available(ctx context.Context) ([]string, error)
	Load(ctx context.Context, model string) error
	Unload(ctx context.Context, model string) error
}

// File is a model file an engine may open, and what it does with it now.
type File struct {
	// Path is the file as the engine resolved it, every link followed.
	Path string `json:"path"`
	// Model is the id the engine serves the file as, once it loads it.
	Model string `json:"model,omitempty"`
	// State is loading, ready, failed or unloading; "" when not loaded.
	State string `json:"state,omitempty"`
	Error string `json:"error,omitempty"`
}

// Begins is an engine whose load and unload answer once the work has
// begun, going on after; the model's state says when it is done.
type Begins interface {
	Begins()
}

// Files is what an adapter offers when its engine loads models by file:
// Load takes the file's path, Unload the model's id.
type Files interface {
	// Files lists the model files the server may open, as it lists them.
	Files(ctx context.Context) ([]File, error)
}
