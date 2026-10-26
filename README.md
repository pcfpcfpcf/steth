# steth

One engine-neutral, zero-config view of a box's models: memory split, KV
fill, speeds, demand, config.

steth reads what the inference engines on a box already report and draws it
on one page: each memory pool (GPU, host memory, drive) divided among the
models that hold it, each model's KV cache, and what each model is doing. It
never loads, unloads or configures a model.

## Engines

| engine | reads | status |
|---|---|---|
| geode | `GET /v1/pool` | pools, per-model memory, KV cache, state, queue; no speed yet |
| Ollama | `GET /api/ps` | per-model GPU and host memory, context length, unload time, model details; no pool sizes, KV split, busy state or speed |
| llama.cpp (`llama serve`) | `GET /props`, `GET /slots`; in router mode `GET /v1/models` and each loaded model through `?model=` | per-model state, KV tokens held, context, running requests, settings; with `--metrics`, decode and prefill speed and requests waiting; no memory figures. Not yet checked against a running server |
| vLLM | | planned |

## Build, run, test

Needs Go 1.24 or later.

    go build ./cmd/steth
    ./steth --target ollama=http://127.0.0.1:11434 --target geode=http://127.0.0.1:8080
    go test ./...

Then open http://127.0.0.1:7474. Click a model for its speeds over the
last two minutes, its requests, and its settings as the engine reports them;
a dash is something its engine does not say, and `derived` marks a number
steth computed from what it does say. `--target ENGINE=URL` repeats for more
engines; `--listen ADDR` moves the page.

Any other tool can read the same state from `GET /api/v1/state`; its shape is
in [docs/schema.md](docs/schema.md).

## Docs

- [docs/design.md](docs/design.md): what it is for, the components and the stages.
- [docs/schema.md](docs/schema.md): the state every engine is read into.
- [LOG.md](LOG.md): decisions as they were made.
- [STYLE.md](STYLE.md): how the code is written.

## License

Apache-2.0.
