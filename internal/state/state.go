// Package state is the engine-neutral shape of a box: its memory pools, the
// models every engine has loaded, and whether each engine answered. It is
// documented in docs/schema.md; fields are added, never renamed.
package state

import "time"

// Schema changes only when a field's meaning does.
const Schema = 1

// Pool names, in the order the page draws them.
const (
	VRAM = "vram"
	DRAM = "dram"
	NVMe = "nvme"
)

var PoolOrder = []string{VRAM, DRAM, NVMe}

// Model states. A reader shows one it does not know by its raw name.
const (
	Serving = "serving"
	Queued  = "queued"
	Moving  = "moving"
	Idle    = "idle"
	// Loaded is a model whose engine does not say whether it is busy.
	Loaded = "loaded"
	// Sleeping is a model whose engine freed its memory while idle and
	// loads it again on the next request.
	Sleeping = "sleeping"
)

// How a non-null field was obtained.
const (
	Measured = "measured"
	Derived  = "derived"
)

type State struct {
	Schema  int      `json:"schema"`
	Pools   []Pool   `json:"pools"`
	Models  []Model  `json:"models"`
	Sources []Source `json:"sources"`
}

// Pool is one kind of memory. CapacityBytes is nil where no size is known,
// as for a drive weights stream from, which holds what it is given.
type Pool struct {
	Name          string  `json:"name"`
	CapacityBytes *uint64 `json:"capacity_bytes"`
}

// Model is one loaded model. A nil field is one its engine does not say.
type Model struct {
	ID         string             `json:"id"`
	Engine     string             `json:"engine"`
	Endpoint   string             `json:"endpoint"`
	State      string             `json:"state"`
	Queued     *int64             `json:"queued"`
	KVTokens   *int64             `json:"kv_tokens"`
	Context    *int64             `json:"context"`
	MaxContext *int64             `json:"max_context"`
	Pools      map[string]Holding `json:"pools"`
	Running    *int64             `json:"running"`
	DecodeTPS  *float64           `json:"decode_tps"`
	PrefillTPS *float64           `json:"prefill_tps"`
	UnloadsAt  *time.Time         `json:"unloads_at"`
	Config     map[string]Setting `json:"config"`
	Fidelity   map[string]string  `json:"fidelity"`
}

// Setting is one launch or load setting as the engine reports it, and
// where it was read.
type Setting struct {
	Value  any    `json:"value"`
	Source string `json:"source"`
}

// Holding is what a model holds in one pool: its weights and buffers, and
// what each token of context reserves for its KV cache. KVBytesPerToken is
// nil where the engine does not split the cache from the rest, which
// WeightBytes then includes.
type Holding struct {
	WeightBytes     uint64   `json:"weight_bytes"`
	KVBytesPerToken *float64 `json:"kv_bytes_per_token"`
}

// Source is one engine endpoint and whether its last read succeeded.
type Source struct {
	Engine   string `json:"engine"`
	Endpoint string `json:"endpoint"`
	OK       bool   `json:"ok"`
	Error    string `json:"error,omitempty"`
}

// Reading is what one engine says: the pools it knows and its models, with
// Engine and Endpoint left for the collector to fill.
type Reading struct {
	Pools  []Pool  `json:"pools"`
	Models []Model `json:"models"`
}
