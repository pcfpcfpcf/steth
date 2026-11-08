package ollama

import (
	"context"

	"github.com/pcfpcfpcf/steth/internal/engine/fetch"
)

type wireTags struct {
	Models []struct {
		Name string `json:"name"`
	} `json:"models"`
}

// Available is every model Ollama has on disk (GET /api/tags).
func (s *Source) Available(ctx context.Context) ([]string, error) {
	var tags wireTags
	if err := fetch.JSON(ctx, s.client, s.endpoint+"/api/tags", &tags); err != nil {
		return nil, err
	}
	names := make([]string, 0, len(tags.Models))
	for _, m := range tags.Models {
		names = append(names, m.Name)
	}
	return names, nil
}

// Load asks for a completion of no prompt, which Ollama answers by loading
// the model and returning once it is in memory.
func (s *Source) Load(ctx context.Context, model string) error {
	return fetch.Post(ctx, s.client, s.endpoint+"/api/generate", map[string]any{"model": model}, nil)
}

// Unload is the same request kept alive for no time, which unloads the
// model.
func (s *Source) Unload(ctx context.Context, model string) error {
	return fetch.Post(ctx, s.client, s.endpoint+"/api/generate", map[string]any{"model": model, "keep_alive": 0}, nil)
}
