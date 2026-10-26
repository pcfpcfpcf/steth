// Package llamacpp reads a llama.cpp server (`llama serve`, formerly
// llama-server): GET /props for the model and whether it sleeps, and
// GET /slots for what each slot's KV cache holds and whether it is busy.
// Started without a model, the server is a router over several, each read
// through `?model=`. llama.cpp says nothing of memory, so a model holds no
// pools until the box layer measures its process.
package llamacpp

import (
	"context"
	"net/http"
	"net/url"

	"github.com/pcfpcfpcf/steth/internal/engine/fetch"
	"github.com/pcfpcfpcf/steth/internal/state"
)

const Engine = "llamacpp"

// Router model statuses (server_model_status_to_string). Any other status
// is a model not in memory: unloaded, downloading, failed.
const (
	routerLoaded   = "loaded"
	routerLoading  = "loading"
	routerSleeping = "sleeping"
)

type Source struct {
	endpoint string
	client   *http.Client
}

func New(endpoint string, client *http.Client) *Source {
	return &Source{endpoint: endpoint, client: client}
}

func (s *Source) Engine() string   { return Engine }
func (s *Source) Endpoint() string { return s.endpoint }

type wireProps struct {
	Role                      string `json:"role"`
	ModelAlias                string `json:"model_alias"`
	EndpointSlots             bool   `json:"endpoint_slots"`
	IsSleeping                bool   `json:"is_sleeping"`
	DefaultGenerationSettings struct {
		NCtx int64 `json:"n_ctx"`
	} `json:"default_generation_settings"`
}

type wireSlot struct {
	IsProcessing  bool  `json:"is_processing"`
	NPromptTokens int64 `json:"n_prompt_tokens"`
}

type wireRouterModels struct {
	Data []struct {
		ID     string `json:"id"`
		Status struct {
			Value string `json:"value"`
		} `json:"status"`
	} `json:"data"`
}

func (s *Source) Read(ctx context.Context) (state.Reading, error) {
	var props wireProps
	if err := s.get(ctx, "/props", "", &props); err != nil {
		return state.Reading{}, err
	}
	if props.Role == "router" {
		return s.readRouter(ctx)
	}
	m, err := s.readModel(ctx, props, "")
	if err != nil {
		return state.Reading{}, err
	}
	return state.Reading{Models: []state.Model{m}}, nil
}

// readRouter reads every model the router has in memory, each through the
// child server that runs it.
func (s *Source) readRouter(ctx context.Context) (state.Reading, error) {
	var list wireRouterModels
	if err := s.get(ctx, "/v1/models", "", &list); err != nil {
		return state.Reading{}, err
	}
	reading := state.Reading{}
	for _, entry := range list.Data {
		switch entry.Status.Value {
		case routerLoaded:
			var props wireProps
			if err := s.get(ctx, "/props", entry.ID, &props); err != nil {
				return state.Reading{}, err
			}
			m, err := s.readModel(ctx, props, entry.ID)
			if err != nil {
				return state.Reading{}, err
			}
			m.ID = entry.ID
			reading.Models = append(reading.Models, m)
		case routerSleeping:
			reading.Models = append(reading.Models, state.Model{ID: entry.ID, State: state.Sleeping, Fidelity: map[string]string{}})
		case routerLoading:
			reading.Models = append(reading.Models, state.Model{ID: entry.ID, State: state.Moving, Fidelity: map[string]string{}})
		}
	}
	return reading, nil
}

// readModel reads one server's model: `model` names it to a router, and is
// empty for a server running one model.
func (s *Source) readModel(ctx context.Context, props wireProps, model string) (state.Model, error) {
	nCtx := props.DefaultGenerationSettings.NCtx
	m := state.Model{
		ID:       props.ModelAlias,
		State:    state.Loaded,
		Context:  &nCtx,
		Fidelity: map[string]string{"context": state.Measured},
	}
	// Reading /slots wakes a sleeping server; a server without it does not
	// say what its slots hold.
	if props.IsSleeping {
		m.State = state.Sleeping
		return m, nil
	}
	if !props.EndpointSlots {
		return m, nil
	}
	var slots []wireSlot
	if err := s.get(ctx, "/slots", model, &slots); err != nil {
		return state.Model{}, err
	}
	// Each slot keeps its last conversation in the cache after it ends.
	var held int64
	m.State = state.Idle
	for _, slot := range slots {
		held += slot.NPromptTokens
		if slot.IsProcessing {
			m.State = state.Serving
		}
	}
	m.KVTokens = &held
	m.Fidelity["kv_tokens"] = state.Measured
	return m, nil
}

// get reads `path`, through the router's child for `model` when it is set.
// autoload=false keeps the router from loading a model to answer.
func (s *Source) get(ctx context.Context, path, model string, into any) error {
	address := s.endpoint + path
	if model != "" {
		address += "?" + url.Values{"model": {model}, "autoload": {"false"}}.Encode()
	}
	return fetch.JSON(ctx, s.client, address, into)
}
