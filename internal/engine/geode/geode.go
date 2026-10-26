// Package geode reads a geode server's GET /v1/pool, which reports every
// pool's capacity and, per model, what it holds in each pool and its KV
// cache.
package geode

import (
	"context"
	"maps"
	"net/http"

	"github.com/pcfpcfpcf/steth/internal/engine/fetch"
	"github.com/pcfpcfpcf/steth/internal/state"
)

const Engine = "geode"

type Source struct {
	endpoint string
	client   *http.Client
}

func New(endpoint string, client *http.Client) *Source {
	return &Source{endpoint: endpoint, client: client}
}

func (s *Source) Engine() string   { return Engine }
func (s *Source) Endpoint() string { return s.endpoint }

type wirePool struct {
	Name          string  `json:"name"`
	CapacityBytes *uint64 `json:"capacity_bytes"`
}

type wireHolding struct {
	Bytes         uint64  `json:"bytes"`
	BytesPerToken float64 `json:"bytes_per_token"`
}

type wireModel struct {
	ID          string                 `json:"id"`
	State       string                 `json:"state"`
	Queued      int64                  `json:"queued"`
	CacheTokens int64                  `json:"cache_tokens"`
	Context     int64                  `json:"context"`
	MostContext int64                  `json:"most_context"`
	Pools       map[string]wireHolding `json:"pools"`
}

type wireStatus struct {
	Pools  []wirePool  `json:"pools"`
	Models []wireModel `json:"models"`
}

// Every field geode reports is read off its own bookkeeping.
var fidelity = map[string]string{
	"queued":      state.Measured,
	"kv_tokens":   state.Measured,
	"context":     state.Measured,
	"max_context": state.Measured,
	"pools":       state.Measured,
}

func (s *Source) Read(ctx context.Context) (state.Reading, error) {
	var status wireStatus
	if err := fetch.JSON(ctx, s.client, s.endpoint+"/v1/pool", &status); err != nil {
		return state.Reading{}, err
	}
	reading := state.Reading{}
	for _, p := range status.Pools {
		reading.Pools = append(reading.Pools, state.Pool{Name: p.Name, CapacityBytes: p.CapacityBytes})
	}
	for _, m := range status.Models {
		reading.Models = append(reading.Models, toModel(m))
	}
	return reading, nil
}

func toModel(m wireModel) state.Model {
	pools := make(map[string]state.Holding, len(m.Pools))
	for name, h := range m.Pools {
		pools[name] = state.Holding{WeightBytes: h.Bytes, KVBytesPerToken: &h.BytesPerToken}
	}
	return state.Model{
		ID:         m.ID,
		State:      m.State,
		Queued:     &m.Queued,
		KVTokens:   &m.CacheTokens,
		Context:    &m.Context,
		MaxContext: &m.MostContext,
		Pools:      pools,
		Fidelity:   maps.Clone(fidelity),
	}
}
