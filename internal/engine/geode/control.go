package geode

import (
	"context"
	"errors"
	"net/http"

	"github.com/pcfpcfpcf/steth/internal/engine/control"
	"github.com/pcfpcfpcf/steth/internal/engine/fetch"
)

// geode loads a model from a file its --models-dir allows: POST
// /models/load {"model": path} answers 202 and /v1/models lists the model
// as loading until it is ready; POST /models/unload {"model": id} drains
// it. A geode from before Nov 2026 has no GET /models/available, and so no
// controls.

type wireAvailable struct {
	Data []struct {
		Path   string `json:"path"`
		Served bool   `json:"served"`
	} `json:"data"`
}

type wireServed struct {
	Data []struct {
		ID    string `json:"id"`
		State string `json:"state"`
		Path  string `json:"path"`
		Error string `json:"error"`
	} `json:"data"`
}

// Files is every file geode may open, with the model each one is now.
func (s *Source) Files(ctx context.Context) ([]control.File, error) {
	files, _, err := s.files(ctx)
	return files, err
}

// files is Files, and the models geode serves.
func (s *Source) files(ctx context.Context) ([]control.File, wireServed, error) {
	var available wireAvailable
	var served wireServed
	if err := fetch.JSON(ctx, s.client, s.endpoint+"/models/available", &available); err != nil {
		var status *fetch.StatusError
		if errors.As(err, &status) && status.Status == http.StatusNotFound {
			return nil, served, control.ErrNone
		}
		return nil, served, err
	}
	if err := fetch.JSON(ctx, s.client, s.endpoint+"/v1/models", &served); err != nil {
		return nil, served, err
	}
	files := make([]control.File, 0, len(available.Data))
	for _, a := range available.Data {
		f := control.File{Path: a.Path}
		for _, m := range served.Data {
			if m.Path != "" && m.Path == a.Path {
				f.Model, f.State, f.Error = m.ID, m.State, m.Error
				if f.State == "" {
					f.State = "ready" // a geode that does not say serves only ready models
				}
			}
		}
		if a.Served && f.State == "" {
			// A geode whose /v1/models names no paths still says it serves
			// the file, though not as which model.
			f.State = "ready"
		}
		files = append(files, f)
	}
	return files, served, nil
}

// Available is every model geode serves, by id, and every file it may open
// and does not, by path: what Unload and Load take.
func (s *Source) Available(ctx context.Context) ([]string, error) {
	files, served, err := s.files(ctx)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(served.Data)+len(files))
	for _, m := range served.Data {
		ids = append(ids, m.ID)
	}
	for _, f := range files {
		if f.State == "" {
			ids = append(ids, f.Path)
		}
	}
	return ids, nil
}

// Begins: geode loads and unloads in the background.
func (s *Source) Begins() {}

// Load asks geode to load the file at path; it answers once the load has
// begun, and the model shows as loading.
func (s *Source) Load(ctx context.Context, path string) error {
	return fetch.Post(ctx, s.client, s.endpoint+"/models/load", map[string]string{"model": path}, nil)
}

// Unload asks geode to drain the model and free its slot.
func (s *Source) Unload(ctx context.Context, model string) error {
	return fetch.Post(ctx, s.client, s.endpoint+"/models/unload", map[string]string{"model": model}, nil)
}
