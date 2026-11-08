// Package geode reads a geode server's GET /v1/pool, which reports every
// pool's capacity and, per model, what it holds in each pool, its KV cache,
// and the tokens it has prefilled and decoded with the seconds they took.
package geode

import (
	"context"
	"maps"
	"net/http"

	"github.com/pcfpcfpcf/steth/internal/engine/fetch"
	"github.com/pcfpcfpcf/steth/internal/state"
)

const Engine = "geode"

// Source reads one server. The collector reads a source from one poll at a
// time, so `last` needs no lock.
type Source struct {
	endpoint string
	client   *http.Client
	// Each model's sums as of the last read, by id.
	last map[string]spent
}

func New(endpoint string, client *http.Client) *Source {
	return &Source{endpoint: endpoint, client: client, last: map[string]spent{}}
}

// spent is what a model's passes have run since its server started.
type spent struct {
	prefillTokens, prefillSeconds, decodeTokens, decodeSeconds float64
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
	// Absent before geode reported them (Nov 8, 2026).
	PrefillTokens  *float64 `json:"prefill_tokens"`
	PrefillSeconds *float64 `json:"prefill_seconds"`
	DecodeTokens   *float64 `json:"decode_tokens"`
	DecodeSeconds  *float64 `json:"decode_seconds"`
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
	next := map[string]spent{}
	for _, wm := range status.Models {
		m := toModel(wm)
		if now, ok := sums(wm); ok {
			next[wm.ID] = now
			if before, ok := s.last[wm.ID]; ok {
				speeds(&m, before, now)
			}
		}
		reading.Models = append(reading.Models, m)
	}
	s.last = next
	return reading, nil
}

// sums is what the model reports it has spent; false from a geode that
// does not report it.
func sums(m wireModel) (spent, bool) {
	if m.PrefillTokens == nil || m.PrefillSeconds == nil || m.DecodeTokens == nil || m.DecodeSeconds == nil {
		return spent{}, false
	}
	return spent{*m.PrefillTokens, *m.PrefillSeconds, *m.DecodeTokens, *m.DecodeSeconds}, true
}

// speeds sets each speed from the tokens and seconds spent since the last
// read: geode's own clock, so a pass that ran across two polls counts once.
func speeds(m *state.Model, before, now spent) {
	if m.DecodeTPS = rate(now.decodeTokens-before.decodeTokens, now.decodeSeconds-before.decodeSeconds); m.DecodeTPS != nil {
		m.Fidelity["decode_tps"] = state.Derived
	}
	if m.PrefillTPS = rate(now.prefillTokens-before.prefillTokens, now.prefillSeconds-before.prefillSeconds); m.PrefillTPS != nil {
		m.Fidelity["prefill_tps"] = state.Derived
	}
}

// rate is tokens over the seconds spent on them; nil when none were spent,
// or when the sums went back, as they do when the server restarts.
func rate(tokens, seconds float64) *float64 {
	if seconds <= 0 || tokens < 0 {
		return nil
	}
	r := tokens / seconds
	return &r
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
