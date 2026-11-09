# steth

One engine-neutral page to watch and manage a box's models: memory split,
KV fill, speeds, demand, config; chat with a model, load and unload models,
start and stop engines.

steth reads what the inference engines on a box already report and draws it
on one page: each memory pool (GPU, host memory, drive) divided among the
models that hold it, each model's KV cache, and what each model is doing.
Watching needs no setup. The person at the box can also act from the page,
always through each engine's own controls and only when asked: try a model
with a prompt, load or unload a model where the engine can, and start or
stop engines from launch profiles in a config file. steth never acts on its
own and never changes an engine's settings.

## Engines

| engine | reads | status |
|---|---|---|
| geode | `GET /v1/pool` | pools, per-model memory, KV cache, state, queue, decode and prefill speed (from geode's `steth` branch on). An internal tool, still not out |
| Ollama | `GET /api/ps` | per-model GPU and host memory, context length, unload time, model details; no pool sizes, KV split, busy state or speed |
| llama.cpp (`llama serve`) | `GET /props`, `GET /slots`; in router mode `GET /v1/models` and each loaded model through `?model=` | per-model state, KV tokens held, context, running requests, settings; with `--metrics`, decode and prefill speed and requests waiting; no memory figures from HTTP; from its process where visible. Checked live with build b10217, router mode included |
| vLLM | `GET /v1/models`, `GET /metrics` | state, requests running and waiting, KV tokens held of the cache's size, decode and prefill speed, cache settings, sleep. Not yet checked against a running server |

## Build, run, test

Needs Go 1.24 or later.

    make        # vet, test (with the race detector), build ./steth
    ./steth

Then open http://127.0.0.1:7474. The page leads with the memory pools and
the models in them; click a model for its speeds over the last two minutes,
its requests, and its settings as the engine reports them; a
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

## Managing

Actions answer only on a loopback `--listen` address (the default), and only
to steth's own page: a request from another site or through a name that is
not this machine's gets 403. Bound elsewhere, steth only watches.

Beside the pools and models, a column holds a toolbar that picks a model
and, with launch profiles, an engine; under it are the conversation and
that model's pulse. The column stays in view as the box scrolls; on a
narrow screen it follows the models.

- **Chat.** Pick a model in the toolbar, or *Talk to it* from its details,
  and send a prompt. It goes to that model
  alone through the OpenAI chat API every engine serves, and the reply
  shows its own numbers: seconds to first text, and decode and prefill
  speed as the engine timed them (geode, llama.cpp) or as steth works them
  out from token counts (`derived`). They describe one reply, so they never
  feed the model's speed, which the pulse charts beside them. Temperature
  and reply length are the engine's own unless set under *Generation
  settings*; each model keeps its own conversation while the page is open.
- **Load and unload.** The toolbar's model list also holds what Ollama, a
  llama.cpp router (`llama serve --models-dir`), or vLLM started with
  `VLLM_SERVER_DEV_MODE=1` could load, marked `not loaded`, and a *Load* or
  *Unload* button for the one picked; vLLM's unload
  is its sleep, which also needs `--enable-sleep-mode`. A model whose engine has no
  controls gets no button.
- **Model files.** *Add a place* under *Model files* browses this machine
  for a directory or a `.gguf` file; every model file under the places is
  listed with its size. A geode started with `--models-dir` for the place
  loads a file with *Load into geode* and unloads it; a file no running
  engine may open has *Start* for each profile that takes model files,
  and else its row says why: a running geode was started without that
  file allowed, or with none (no `--models-dir`).
  Places added on the page are kept in `--places`, by default
  `steth/places.json` in your config directory.
- **Start and stop engines.** `--config steth.json` names launch profiles
  and where model files are:

      {"models": ["/models", "/data/Qwen3.5-9B-Q4_K_M.gguf"],
       "launch": [
        {"name": "geode", "command": ["geode", "serve", "{model}", "--models-dir", "{place}", "--port", "8080"]},
        {"name": "llama router", "command": ["llama", "serve", "--models-dir", "/models", "--port", "9931"],
         "dir": "/srv/llama", "env": {"LLAMA_CACHE": "/srv/cache"}}
      ]}

  The toolbar's *Engine* picks a profile and *With* its `{model}`: the
  profile's own `models`, or else every `.gguf` file under the places: the
  top-level `models` (directories and files) and those added on the page,
  found again at each look and shown with its size. An argument with
  `{place}` repeats for each place, after the flag before it. The engine's log is under the pulse. steth refuses to start a
  profile already running or whose `--port` already answers, keeps each
  engine's last 120 log lines, stops only engines it started, and stops all
  of them when it exits. A started engine appears like any other, through
  discovery. The page never supplies a command.

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
