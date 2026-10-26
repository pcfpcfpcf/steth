# Design

steth is a doctor for local inference engines: one page that shows what each
engine on a box has loaded and where, how fast it runs, how much demand it
gets and how it is configured.

Principle: **read what the engines already say, show it in one engine-neutral
shape, and never invent a number.**

## What it is not

- Not a model manager: it never loads, unloads or configures a model.
- Not a time-series database: it keeps a few minutes in memory for
  sparklines. Grafana and Prometheus own long-term history.
- Not a proxy: it reads engines' own endpoints and the operating system.

## Data flow

```
engines (geode, ollama, llama.cpp, vLLM)      operating system
        |  HTTP, each engine's own API              |  /proc, nvidia-smi
        v                                           v
   adapters (one per engine)                   box layer
        \                                          /
         +------------> collector <---------------+
                        polls every 1 s, merges into one State
                            |
                            v
                web: GET /api/v1/state, GET / (the page)
```

## Components

Each lives in its own package and is replaceable alone.

| package | job | surface |
|---|---|---|
| `internal/state` | the schema: the shape every adapter fills and the page reads | types only |
| `internal/engine/<name>` | reads one engine's endpoints into a `state.Reading` | `New(endpoint)`, `Read(ctx)` |
| `internal/collector` | polls every source on a monotonic schedule, merges readings, keeps the latest state | `New(sources)`, `Run(ctx)`, `State()` |
| `internal/web` | serves the embedded page and the state as JSON | `Handler(state func) http.Handler` |
| `cmd/steth` | flags and wiring | the binary |

Layers point down: `cmd` knows everything; `web` and `collector` know
`state`; adapters know `state` and their engine; `state` knows nothing.

The collector stamps each model with its source's engine and endpoint, so an
adapter never repeats them. Pools are merged by name; the first source that
knows a pool's capacity sets it.

## Stages

1. Skeleton, schema, collector, geode adapter, page ported.
2. Ollama adapter.
3. llama.cpp adapter.
4. Speed, demand and config, through every adapter that has them.
5. Discovery (find engines on local ports) and box layer (pool capacities,
   per-process VRAM and RSS).
6. vLLM adapter.
7. Docs, release binaries, CI.

## Out of scope for now

Remote boxes beyond `--target`, authentication, Prometheus output, alerting.
