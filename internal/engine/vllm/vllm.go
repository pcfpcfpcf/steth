// Package vllm reads a vLLM server: GET /v1/models for the model it serves
// and GET /metrics for its requests, its KV cache, its sleep state and its
// token counters. A server serves one base model; LoRA adapters on it are
// read as part of it.
package vllm

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/pcfpcfpcf/steth/internal/engine/fetch"
	"github.com/pcfpcfpcf/steth/internal/engine/local"
	"github.com/pcfpcfpcf/steth/internal/engine/prom"
	"github.com/pcfpcfpcf/steth/internal/state"
)

const Engine = "vllm"

// The cache_config_info labels shown as config, under vLLM's own names.
var cacheSettings = []string{
	"block_size", "num_gpu_blocks", "kv_cache_size_tokens", "gpu_memory_utilization",
	"cache_dtype", "enable_prefix_caching", "sliding_window",
}

// Source reads one server. The collector reads a source from one poll at a
// time, so `last` needs no lock.
type Source struct {
	endpoint string
	client   *http.Client
	now      func() time.Time
	last     map[string]totals
	local    *local.Finder
}

// totals are a model's token counters when they were read.
type totals struct {
	at              time.Time
	prompt, decoded float64
}

func New(endpoint string, client *http.Client) *Source {
	return &Source{endpoint: endpoint, client: client, now: time.Now, last: map[string]totals{}, local: local.NewFinder(Runs)}
}

func (s *Source) Engine() string   { return Engine }
func (s *Source) Endpoint() string { return s.endpoint }

type wireModels struct {
	Data []struct {
		ID          string  `json:"id"`
		OwnedBy     string  `json:"owned_by"`
		Root        string  `json:"root"`
		Parent      *string `json:"parent"`
		MaxModelLen *int64  `json:"max_model_len"`
	} `json:"data"`
}

func (s *Source) Read(ctx context.Context) (state.Reading, error) {
	var models wireModels
	if err := fetch.JSON(ctx, s.client, s.endpoint+"/v1/models", &models); err != nil {
		return state.Reading{}, err
	}
	body, err := fetch.Body(ctx, s.client, s.endpoint+"/metrics")
	if err != nil {
		return state.Reading{}, err
	}
	samples, err := prom.Parse(body)
	if err != nil {
		return state.Reading{}, err
	}
	now := s.now()
	next := map[string]totals{}
	reading := state.Reading{}
	for _, card := range models.Data {
		if card.Parent != nil {
			continue
		}
		m := s.toModel(samples, card.ID, now, next)
		// max_model_len caps one request; context is the cache all of them
		// share, so it is a setting, not the model's max_context.
		if card.MaxModelLen != nil {
			m.Config["max_model_len"] = state.Setting{Value: *card.MaxModelLen, Source: "/v1/models"}
		}
		m.Config["root"] = state.Setting{Value: card.Root, Source: "/v1/models"}
		reading.Models = append(reading.Models, m)
	}
	if len(reading.Models) > 0 {
		if err := s.readProcess(ctx, &reading.Models[0]); err != nil {
			return state.Reading{}, err
		}
	}
	s.last = next
	return reading, nil
}

// readProcess fills the served model from the server's processes, where they
// can be seen: what they hold in each pool, the engine core that holds the
// weights and cache included, and the command line.
func (s *Source) readProcess(ctx context.Context, m *state.Model) error {
	proc := s.local.At(local.Port(s.endpoint))
	if proc == nil {
		return nil
	}
	held, err := s.local.Processes.Memory(ctx, proc.PID)
	if err != nil {
		return err
	}
	m.Pools = map[string]state.Holding{}
	for pool, bytes := range held {
		m.Pools[pool] = state.Holding{WeightBytes: bytes}
	}
	m.Fidelity["pools"] = state.Measured
	m.Config["launch args"] = state.Setting{Value: proc.Args, Source: "launch args"}
	return nil
}

// toModel reads the metrics labelled with `name`, and the cache settings,
// which describe the whole server. Counters land in `next`; speeds come from
// their change since the last read, over the time between the reads.
func (s *Source) toModel(samples []prom.Sample, name string, now time.Time, next map[string]totals) state.Model {
	of := map[string]string{"model_name": name}
	m := state.Model{ID: name, State: state.Idle, Fidelity: map[string]string{}, Config: map[string]state.Setting{}}
	measure := func(metric, field string, into **int64) {
		if v, ok := prom.Sum(samples, metric, of); ok {
			n := int64(v)
			*into = &n
			m.Fidelity[field] = state.Measured
		}
	}
	measure("vllm:num_requests_running", "running", &m.Running)
	measure("vllm:num_requests_waiting", "queued", &m.Queued)
	switch {
	case m.Running != nil && *m.Running > 0:
		m.State = state.Serving
	case m.Queued != nil && *m.Queued > 0:
		m.State = state.Queued
	}
	if awake, ok := prom.Sum(samples, "vllm:engine_sleep_state", map[string]string{"model_name": name, "sleep_state": "awake"}); ok && awake == 0 {
		m.State = state.Sleeping
	}

	info := cacheInfo(samples)
	for _, setting := range cacheSettings {
		if v, ok := info[setting]; ok && v != "None" {
			m.Config[setting] = state.Setting{Value: v, Source: "/metrics"}
		}
	}
	if capacity := kvCapacity(info); capacity > 0 {
		m.Context = &capacity
		m.Fidelity["context"] = state.Measured
		if used, ok := prom.Sum(samples, "vllm:kv_cache_usage_perc", of); ok {
			held := int64(used*float64(capacity) + 0.5)
			m.KVTokens = &held
			m.Fidelity["kv_tokens"] = state.Derived
		}
	}

	prompt, okPrompt := prom.Sum(samples, "vllm:prompt_tokens_total", of)
	decoded, okDecoded := prom.Sum(samples, "vllm:generation_tokens_total", of)
	if !okPrompt || !okDecoded {
		return m
	}
	next[name] = totals{at: now, prompt: prompt, decoded: decoded}
	if before, ok := s.last[name]; ok {
		seconds := now.Sub(before.at).Seconds()
		if m.DecodeTPS = rate(decoded-before.decoded, seconds); m.DecodeTPS != nil {
			m.Fidelity["decode_tps"] = state.Derived
		}
		if m.PrefillTPS = rate(prompt-before.prompt, seconds); m.PrefillTPS != nil {
			m.Fidelity["prefill_tps"] = state.Derived
		}
	}
	return m
}

// rate is tokens per second of wall time; nil when no token moved, or when
// the counters went back, as they do when the server restarts.
func rate(tokens, seconds float64) *float64 {
	if tokens <= 0 || seconds <= 0 {
		return nil
	}
	r := tokens / seconds
	return &r
}

// cacheInfo is the labels of vllm:cache_config_info, the server's cache
// settings; engine 0's where data parallelism runs several.
func cacheInfo(samples []prom.Sample) map[string]string {
	for _, s := range samples {
		if s.Name == "vllm:cache_config_info" && (s.Labels["engine"] == "0" || s.Labels["engine"] == "") {
			return s.Labels
		}
	}
	return nil
}

// kvCapacity is how many tokens the KV cache holds: as vLLM sizes it, or
// failing that its blocks times the tokens in each.
func kvCapacity(info map[string]string) int64 {
	if tokens, err := strconv.ParseInt(info["kv_cache_size_tokens"], 10, 64); err == nil {
		return tokens
	}
	blocks, errBlocks := strconv.ParseInt(info["num_gpu_blocks"], 10, 64)
	size, errSize := strconv.ParseInt(info["block_size"], 10, 64)
	if errBlocks != nil || errSize != nil {
		return 0
	}
	return blocks * size
}
