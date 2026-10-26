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
| `internal/engine/<name>` | reads one engine's endpoints into a `state.Reading`; says which command lines run it, its default ports, and how to probe it | `New(endpoint)`, `Read(ctx)`, `Runs`, `Ports`, `Probe` |
| `internal/engine/fetch` | GETs an engine's JSON or text | `JSON`, `Body` |
| `internal/engine/enginetest` | serves recorded responses, compares readings with expected JSON | `Serve`, `Match` |
| `internal/box` | what the OS says: pool sizes and use, a process's memory, command line and parent, listening sockets; also a source of pools | `Source`, `ProcessMemory`, `Listeners`, `Cmdline`, `ParentPID` |
| `internal/discover` | finds engines from sockets, processes and probes; keeps the source list | `Find`, `Registry` |
| `internal/collector` | polls whichever sources there are on a monotonic schedule, merges readings, keeps the latest state and two minutes of speeds | `New(sources func)`, `Run(ctx)`, `State()` |
| `internal/web` | serves the embedded page and the state as JSON | `Handler(state func) http.Handler` |
| `cmd/steth` | flags, the engine list, and wiring | the binary |

Layers point down: `cmd` knows everything; `discover` knows `box` and the
collector's `Source`; `web` and `collector` know `state`; adapters know
`state`, `fetch`, `box` and their engine; `state` knows nothing.

The collector stamps each model with its source's engine and endpoint, so an
adapter never repeats them. Pools are merged by name; the first source that
knows a pool's capacity, or what is in use of it, sets it. The box is read
first, so the machine's own figures win.

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
