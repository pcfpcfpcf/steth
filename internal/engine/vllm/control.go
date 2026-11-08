package vllm

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"

	"github.com/pcfpcfpcf/steth/internal/engine/control"
	"github.com/pcfpcfpcf/steth/internal/engine/fetch"
)

// A vLLM process serves one model for life, so loading and unloading are
// waking it and putting it to sleep. The routes exist only on a server run
// with VLLM_SERVER_DEV_MODE=1; putting it to sleep works only with
// --enable-sleep-mode. Only GET /server_info says whether that is on, and it
// gathers the whole environment to answer, so it is not asked.

// Available is the model the process serves, when it has the sleep routes.
func (s *Source) Available(ctx context.Context) ([]string, error) {
	if _, err := fetch.Body(ctx, s.client, s.endpoint+"/is_sleeping"); err != nil {
		return nil, withoutRoutes(err)
	}
	return s.served(ctx)
}

// Load wakes the model: its weights back on the GPU, its KV cache made
// again.
func (s *Source) Load(ctx context.Context, model string) error {
	if err := s.mustServe(ctx, model); err != nil {
		return err
	}
	return withoutRoutes(fetch.Post(ctx, s.client, s.endpoint+"/wake_up", nil, nil))
}

// Unload puts the model to sleep at level 1: weights moved to host memory,
// KV cache dropped. Requests in flight are aborted, vLLM's default, as
// llama.cpp's unload stops them too.
func (s *Source) Unload(ctx context.Context, model string) error {
	if err := s.mustServe(ctx, model); err != nil {
		return err
	}
	return withoutRoutes(fetch.Post(ctx, s.client, s.endpoint+"/sleep?level=1", nil, nil))
}

// served is the base model's id; LoRA adapters on it are not loaded alone.
func (s *Source) served(ctx context.Context) ([]string, error) {
	var models wireModels
	if err := fetch.JSON(ctx, s.client, s.endpoint+"/v1/models", &models); err != nil {
		return nil, err
	}
	var ids []string
	for _, card := range models.Data {
		if card.Parent == nil {
			ids = append(ids, card.ID)
		}
	}
	return ids, nil
}

// mustServe refuses a model this process does not serve.
func (s *Source) mustServe(ctx context.Context, model string) error {
	ids, err := s.served(ctx)
	if err != nil {
		return err
	}
	if slices.Contains(ids, model) {
		return nil
	}
	return fmt.Errorf("vLLM at %s serves %q, not %q", s.endpoint, ids, model)
}

// withoutRoutes is control.ErrNone for the 404 of a server not in dev mode.
func withoutRoutes(err error) error {
	var status *fetch.StatusError
	if errors.As(err, &status) && status.Status == http.StatusNotFound {
		return control.ErrNone
	}
	return err
}
