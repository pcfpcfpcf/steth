// Package ollama reads an Ollama server's GET /api/ps: the models it has
// loaded, how much of each is on the GPU, and each one's context length.
// Ollama says neither how big the pools are, nor how much of a model is its
// KV cache, nor whether a model is busy.
package ollama

import (
	"context"
	"net/http"
	"time"

	"github.com/pcfpcfpcf/steth/internal/engine/fetch"
	"github.com/pcfpcfpcf/steth/internal/state"
)

const Engine = "ollama"

type Source struct {
	endpoint string
	client   *http.Client
	now      func() time.Time
}

func New(endpoint string, client *http.Client) *Source {
	return &Source{endpoint: endpoint, client: client, now: time.Now}
}

func (s *Source) Engine() string   { return Engine }
func (s *Source) Endpoint() string { return s.endpoint }

type wireModel struct {
	Name          string    `json:"name"`
	Size          uint64    `json:"size"`
	SizeVRAM      uint64    `json:"size_vram"`
	ContextLength *int64    `json:"context_length"`
	ExpiresAt     time.Time `json:"expires_at"`
	Details       struct {
		Format            string `json:"format"`
		Family            string `json:"family"`
		ParameterSize     string `json:"parameter_size"`
		QuantizationLevel string `json:"quantization_level"`
	} `json:"details"`
}

// A model kept loaded for good (keep_alive below zero) expires centuries
// ahead; anything past this is "never".
const neverUnloads = 10 * 365 * 24 * time.Hour

type wireStatus struct {
	Models []wireModel `json:"models"`
}

func (s *Source) Read(ctx context.Context) (state.Reading, error) {
	var status wireStatus
	if err := fetch.JSON(ctx, s.client, s.endpoint+"/api/ps", &status); err != nil {
		return state.Reading{}, err
	}
	reading := state.Reading{}
	held := map[string]bool{}
	for _, w := range status.Models {
		m := toModel(w, s.now())
		for name, h := range m.Pools {
			held[name] = held[name] || h.WeightBytes > 0
		}
		reading.Models = append(reading.Models, m)
	}
	// Only the pools a model is in: a box without a GPU shows none.
	for _, name := range state.PoolOrder {
		if held[name] {
			reading.Pools = append(reading.Pools, state.Pool{Name: name})
		}
	}
	return reading, nil
}

// toModel maps one entry of /api/ps; `now` decides whether its expiry is
// real.
func toModel(w wireModel, now time.Time) state.Model {
	fidelity := map[string]string{"pools": state.Measured}
	if w.ContextLength != nil {
		fidelity["context"] = state.Measured
	}
	var unloadsAt *time.Time
	if !w.ExpiresAt.IsZero() && w.ExpiresAt.Sub(now) < neverUnloads {
		unloadsAt = &w.ExpiresAt
		fidelity["unloads_at"] = state.Measured
	}
	config := map[string]state.Setting{}
	for name, value := range map[string]string{
		"format":             w.Details.Format,
		"family":             w.Details.Family,
		"parameter_size":     w.Details.ParameterSize,
		"quantization_level": w.Details.QuantizationLevel,
	} {
		if value != "" {
			config[name] = state.Setting{Value: value, Source: "/api/ps"}
		}
	}
	return state.Model{
		ID:      w.Name,
		State:   state.Loaded,
		Context: w.ContextLength,
		// What is not on the GPU is in host memory: Ollama does not stream
		// from a drive.
		Pools: map[string]state.Holding{
			state.VRAM: {WeightBytes: w.SizeVRAM},
			state.DRAM: {WeightBytes: w.Size - min(w.SizeVRAM, w.Size)},
		},
		UnloadsAt: unloadsAt,
		Config:    config,
		Fidelity:  fidelity,
	}
}
