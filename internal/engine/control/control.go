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
