package llamacpp

import (
	"context"

	"github.com/pcfpcfpcf/steth/internal/engine/control"
	"github.com/pcfpcfpcf/steth/internal/engine/fetch"
)

// Only a router loads and unloads; a server of one model has no controls.

// Available is every model the router knows (GET /models, the handler
// /v1/models also runs), loaded or not.
func (s *Source) Available(ctx context.Context) ([]string, error) {
	if err := s.mustRoute(ctx); err != nil {
		return nil, err
	}
	var list wireRouterModels
	if err := s.get(ctx, "/models", "", &list); err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(list.Data))
	for _, entry := range list.Data {
		ids = append(ids, entry.ID)
	}
	return ids, nil
}

// Load starts the model's server and returns at once; the router lists it
// as loading until it is ready.
func (s *Source) Load(ctx context.Context, model string) error {
	return s.post(ctx, "/models/load", model)
}

// Unload stops the model's server.
func (s *Source) Unload(ctx context.Context, model string) error {
	return s.post(ctx, "/models/unload", model)
}

func (s *Source) post(ctx context.Context, path, model string) error {
	if err := s.mustRoute(ctx); err != nil {
		return err
	}
	return fetch.Post(ctx, s.client, s.endpoint+path, map[string]string{"model": model}, nil)
}

// mustRoute fails with control.ErrNone unless the server is a router.
func (s *Source) mustRoute(ctx context.Context) error {
	var props wireProps
	if err := s.get(ctx, "/props", "", &props); err != nil {
		return err
	}
	if props.Role != "router" {
		return control.ErrNone
	}
	return nil
}
