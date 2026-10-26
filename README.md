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
| llama.cpp (`llama serve`) | `GET /props`, `GET /slots`; in router mode `GET /v1/models` and each loaded model through `?model=` | per-model state, KV tokens held, context, running requests, settings; with `--metrics`, decode and prefill speed and requests waiting; no memory figures from HTTP; from its process where visible. Checked live with build b10217, router mode included |
| vLLM | `GET /v1/models`, `GET /metrics` | state, requests running and waiting, KV tokens held of the cache's size, decode and prefill speed, cache settings, sleep. Not yet checked against a running server |

## Build, run, test

Needs Go 1.24 or later.

    make        # vet, test (with the race detector), build ./steth
    ./steth

Then open http://127.0.0.1:7474. Click a model for its speeds over the last
two minutes, its requests, and its settings as the engine reports them; a
dash is something its engine does not say, and `derived` marks a number
steth computed from what it does say.

steth finds the engines on its machine by itself: every listening socket
whose process runs an engine (`ollama serve`, `llama serve`, `vllm serve`,
`geode serve`),
asked as that engine to confirm, and, where the process is another user's,
such as a system service, the engines' default ports. It looks again every
10 seconds. `--target ENGINE=URL` adds one it cannot find, such as an engine
on another machine; repeat it for more. `-discover=false` reads only those.
`--listen ADDR` moves the page.

steth also reads the machine itself: each pool's size and how much of it is
in use, and, for llama.cpp and vLLM servers it can see, what their processes
hold and how they were launched. On Linux from `/proc` and `nvidia-smi`
; on macOS from `lsof`, `ps`, `sysctl` and `vm_stat`; on Windows from the
Windows API, `netstat` and PowerShell. CI runs the tests that read the
machine on all three.

Any other tool can read the same state from `GET /api/v1/state`; its shape is
in [docs/schema.md](docs/schema.md).

To work on the page without running an engine, `cmd/mockengine` answers as
one from recorded responses (each adapter's `testdata/`); its doc comment
shows how.

## Release

Add a `## vX.Y.Z` section to [CHANGELOG.md](CHANGELOG.md), then push the tag
`vX.Y.Z`: CI builds Linux, macOS and Windows binaries for amd64 and arm64
and publishes them with that section as the notes. `scripts/release.sh
vX.Y.Z` builds the same archives into `dist/` locally.

## Docs

- [docs/design.md](docs/design.md): what it is for, the components and the stages.
- [docs/schema.md](docs/schema.md): the state every engine is read into.
- [LOG.md](LOG.md): decisions as they were made.
- [CHANGELOG.md](CHANGELOG.md): what each version changed.
- [STYLE.md](STYLE.md): how the code is written.

## License

Apache-2.0.
