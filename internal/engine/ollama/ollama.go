// Package ollama reads an Ollama server's GET /api/ps: the models it has
// loaded, how much of each is on the GPU, and each one's context length.
// Ollama says neither how big the pools are, nor how much of a model is its
// KV cache, nor whether a model is busy.
package ollama

import (
	"context"
	"net/http"

	"github.com/pcfpcfpcf/steth/internal/engine/fetch"
	"github.com/pcfpcfpcf/steth/internal/state"
)

const Engine = "ollama"

type Source struct {
	endpoint string
	client   *http.Client
}

func New(endpoint string, client *http.Client) *Source {
	return &Source{endpoint: endpoint, client: client}
}

func (s *Source) Engine() string   { return Engine }
func (s *Source) Endpoint() string { return s.endpoint }

type wireModel struct {
	Name          string `json:"name"`
	Size          uint64 `json:"size"`
	SizeVRAM      uint64 `json:"size_vram"`
	ContextLength *int64 `json:"context_length"`
}

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
		m := toModel(w)
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

func toModel(w wireModel) state.Model {
	fidelity := map[string]string{"pools": state.Measured}
	if w.ContextLength != nil {
		fidelity["context"] = state.Measured
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
		Fidelity: fidelity,
	}
}
