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
	"github.com/pcfpcfpcf/steth/internal/engine/local"
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

// Source reads one server. The collector reads a source from one poll at a
// time, so `last` and `next` need no lock.
type Source struct {
	endpoint string
	client   *http.Client
	// Each model's counters as of the last read, and as of this one; keyed
	// by the name a router knows it by, "" for a server of one model.
	last, next map[string]counters
	// The local server processes, by the port each listens on.
	local *local.Finder
}

func New(endpoint string, client *http.Client) *Source {
	return &Source{endpoint: endpoint, client: client, last: map[string]counters{}, local: local.NewFinder(Runs)}
}

func (s *Source) Engine() string   { return Engine }
func (s *Source) Endpoint() string { return s.endpoint }

type wireProps struct {
	Role                      string `json:"role"`
	ModelAlias                string `json:"model_alias"`
	EndpointSlots             bool   `json:"endpoint_slots"`
	EndpointMetrics           bool   `json:"endpoint_metrics"`
	TotalSlots                int64  `json:"total_slots"`
	ModelPath                 string `json:"model_path"`
	ModelFtype                string `json:"model_ftype"`
	BuildInfo                 string `json:"build_info"`
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
			Value string   `json:"value"`
			Args  []string `json:"args"`
		} `json:"status"`
	} `json:"data"`
}

// Read reads the server; the counters of models it no longer has are
// dropped with the read that misses them.
func (s *Source) Read(ctx context.Context) (state.Reading, error) {
	s.next = map[string]counters{}
	reading, err := s.read(ctx)
	if err == nil {
		s.last = s.next
	}
	return reading, err
}

func (s *Source) read(ctx context.Context) (state.Reading, error) {
	var props wireProps
	if err := s.get(ctx, "/props", "", &props); err != nil {
		return state.Reading{}, err
	}
	if props.Role == "router" {
		return s.readRouter(ctx)
	}
	m, err := s.readModel(ctx, props, "", s.local.At(local.Port(s.endpoint)))
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
			// A router's servers run beside it, each on the port it was
			// started with.
			var child *local.Server
			if local.Port(s.endpoint) != 0 {
				child = s.local.At(launch(entry.Status.Args).port())
			}
			m, err := s.readModel(ctx, props, entry.ID, child)
			if err != nil {
				return state.Reading{}, err
			}
			m.ID = entry.ID
			m.Config["args"] = argsOf(entry.Status.Args)
			reading.Models = append(reading.Models, m)
		case routerSleeping, routerLoading:
			m := state.Model{ID: entry.ID, State: state.Sleeping, Fidelity: map[string]string{}}
			if entry.Status.Value == routerLoading {
				m.State = state.Moving
			}
			m.Config = map[string]state.Setting{"args": argsOf(entry.Status.Args)}
			reading.Models = append(reading.Models, m)
		}
	}
	return reading, nil
}

// readModel reads one server's model: `model` names it to a router, and is
// empty for a server running one model. `proc` is the server's process
// where it can be seen, nil otherwise.
func (s *Source) readModel(ctx context.Context, props wireProps, model string, proc *local.Server) (state.Model, error) {
	nCtx := props.DefaultGenerationSettings.NCtx
	m := state.Model{
		ID:       props.ModelAlias,
		State:    state.Loaded,
		Context:  &nCtx,
		Config:   configOf(props),
		Fidelity: map[string]string{"context": state.Measured},
	}
	if proc != nil {
		if err := s.readProcess(ctx, props, proc, &m); err != nil {
			return state.Model{}, err
		}
	}
	// Reading /slots wakes a sleeping server; /metrics does not, but a
	// sleeping server has nothing running.
	if props.IsSleeping {
		m.State = state.Sleeping
		return m, nil
	}
	if props.EndpointMetrics {
		if err := s.readMetrics(ctx, model, &m); err != nil {
			return state.Model{}, err
		}
	}
	// Reading /slots also restarts the idle timer, so a server that sleeps
	// when idle would never sleep while steth watches it.
	if !props.EndpointSlots || (proc != nil && launch(proc.Args).sleeps()) {
		if m.Running != nil {
			m.State = state.Idle
			if *m.Running > 0 {
				m.State = state.Serving
			}
		}
		return m, nil
	}
	var slots []wireSlot
	if err := s.get(ctx, "/slots", model, &slots); err != nil {
		return state.Model{}, err
	}
	// Each slot keeps its last conversation in the cache after it ends.
	var held, busy int64
	for _, slot := range slots {
		held += slot.NPromptTokens
		if slot.IsProcessing {
			busy++
		}
	}
	m.State = state.Idle
	if busy > 0 {
		m.State = state.Serving
	}
	m.KVTokens = &held
	m.Fidelity["kv_tokens"] = state.Measured
	if m.Running == nil {
		m.Running = &busy
		m.Fidelity["running"] = state.Measured
	}
	return m, nil
}

// get decodes the JSON at `path` into `into`.
func (s *Source) get(ctx context.Context, path, model string, into any) error {
	return fetch.JSON(ctx, s.client, s.address(path, model), into)
}

// address is `path` on the server, through the router's child for `model`
// when it is set. autoload=false keeps the router from loading a model to
// answer.
func (s *Source) address(path, model string) string {
	if model == "" {
		return s.endpoint + path
	}
	return s.endpoint + path + "?" + url.Values{"model": {model}, "autoload": {"false"}}.Encode()
}

// configOf is the server's settings as /props reports them, under the
// names it uses.
func configOf(props wireProps) map[string]state.Setting {
	values := map[string]any{
		"n_ctx":            props.DefaultGenerationSettings.NCtx,
		"total_slots":      props.TotalSlots,
		"model_path":       props.ModelPath,
		"model_ftype":      props.ModelFtype,
		"endpoint_slots":   props.EndpointSlots,
		"endpoint_metrics": props.EndpointMetrics,
		"build_info":       props.BuildInfo,
	}
	config := make(map[string]state.Setting, len(values))
	for name, value := range values {
		config[name] = state.Setting{Value: value, Source: "/props"}
	}
	return config
}

// argsOf is the command line a router started a model's server with.
func argsOf(args []string) state.Setting {
	return state.Setting{Value: args, Source: "/v1/models"}
}

// readProcess fills `m` from the server's process: what it holds in each
// pool, its command line, and the KV cache's whole size where the slots
// split it rather than share it.
func (s *Source) readProcess(ctx context.Context, props wireProps, proc *local.Server, m *state.Model) error {
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
	if !launch(proc.Args).unifiedKV() && props.TotalSlots > 1 {
		whole := props.DefaultGenerationSettings.NCtx * props.TotalSlots
		m.Context = &whole
		m.Fidelity["context"] = state.Derived
	}
	return nil
}
