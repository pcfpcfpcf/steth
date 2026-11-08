# Design

steth is a doctor for local inference engines: one page that shows what each
engine on a box has loaded and where, how fast it runs, how much demand it
gets and how it is configured, and lets the person at the box start engines,
load and unload models, and try a model with a prompt.

Principle: **read what the engines already say and never invent a number; act
only through each engine's own controls, and only when someone asks.**

Stages 1–7 (monitoring) and 8–11 (managing: launching engines, chat with
per-reply timings, load and unload, none of it tied to one engine) are
built. The page keeps its placement view and model cards on the left; the
controls sit beside them in one workspace (see "The page").

## What it is not

- Not an autopilot: it never loads, unloads, starts or stops anything on its
  own. Every action is a click, or a call to its API, by someone at the box.
- Not a model downloader: it launches and loads what is already on disk or
  already known to an engine.
- Not a router or load balancer: it does not sit in the path of other
  clients' requests. It proxies only the prompts sent from its own page, so
  the speeds it reads for all traffic still come from the engines.
- Not a remote control: actions answer only on a loopback address, from its
  own page. Bound elsewhere with `--listen`, steth only monitors.
- Not a time-series database: it keeps a few minutes in memory for
  sparklines. Grafana and Prometheus own long-term history.

## Data flow

```
engines (geode, ollama, llama.cpp, vLLM)        operating system
   ^   |  HTTP, each engine's own API                |  /proc, nvidia-smi
   |   v                                             v
   |  adapters: read; control where the engine can  box layer
   |       \                                         /
   |        +-----------> collector <---------------+
   |                      polls every 1 s, merges into one State
   |                          |
   |  launch: starts and stops engine processes from profiles;
   |  discovery finds them like any other engine
   |                          |
   +---- web: GET /api/v1/state, GET / (the page);
              actions: chat, load, unload, launch (loopback, own page only)
```

## Components

Each lives in its own package and is replaceable alone.

| package | job | surface |
|---|---|---|
| `internal/state` | the schema: the shape every adapter fills and the page reads | types only |
| `internal/engine/<name>` | reads one engine's endpoints into a `state.Reading`; says which command lines run it, its default ports, and how to probe it; where the engine has the controls, lists the models it could load and loads or unloads one | `New(endpoint)`, `Read(ctx)`, `Runs`, `Ports`, `Probe`; optional `control.Controls` |
| `internal/engine/control` | the optional interface an adapter with load and unload implements | `Controls`: `Available(ctx)`, `Load(ctx, id)`, `Unload(ctx, id)`; `ErrNone` |
| `internal/engine/fetch` | GETs an engine's JSON or text; POSTs JSON; a refused request's error carries the status and the engine's own message | `JSON`, `Body`, `Post`, `StatusError` |
| `internal/engine/enginetest` | serves recorded responses, compares readings with expected JSON | `Serve`, `Match` |
| `internal/box` | what the OS says: pool sizes and use, a process's memory, command line and parent, listening sockets; also a source of pools | `Source`, `ProcessMemory`, `Listeners`, `Cmdline`, `ParentPID` |
| `internal/discover` | finds engines from sockets, processes and probes; keeps the source list | `Find`, `Registry` |
| `internal/collector` | polls whichever sources there are on a monotonic schedule, merges readings, keeps the latest state and two minutes of speeds | `New(sources func)`, `Run(ctx)`, `State()` |
| `internal/chat` | sends one prompt to a model steth knows, streams the reply back, and reports that reply's numbers | `Send(ctx, target, request, w) (Reply, error)` |
| `internal/library` | the places of model files (the config's, and those added from the page, kept in a file); the `.gguf` files under them; one directory listed for picking a place | `Open(fixed, file)`, `Add`, `Remove`, `Scan(places)`, `Browse(dir)` |
| `internal/launch` | reads the config file; starts and stops engine processes from its profiles, a `{model}` among the files under the places; owns only its children; keeps each one's recent log | `Load(path)`, `New(profiles, places)`, `Start(name, model)`, `Stop(name)`, `Status()` |
| `internal/web` | serves the embedded page and the state as JSON; takes actions from its own page only | `Handler(state func, *Actions) http.Handler`, `Loopback(listen)` |
| `cmd/steth` | flags, the config file, the engine list, and wiring | the binary |

Layers point down: `cmd` knows everything; `discover` knows `box` and the
collector's `Source`; `web` knows `state`, `chat` and `launch`, and reaches
adapters only through the interfaces above; `collector` knows `state`;
adapters know `state`, `fetch`, `box` and their engine; `launch` knows only
the OS; `state` knows nothing.

The collector stamps each model with its source's engine and endpoint, so an
adapter never repeats them. Pools are merged by name; the first source that
knows a pool's capacity, or what is in use of it, sets it. The box is read
first, so the machine's own figures win.

## Managing

### Load and unload, through the engine

Only engines that can do it over HTTP get buttons; the page shows no button
for the rest, rather than one that fails.

| engine | available models | load | unload |
|---|---|---|---|
| Ollama | `GET /api/tags` | `POST /api/generate` with the model and no prompt; returns once loaded | the same with `keep_alive: 0`; returns at once, gone from `/api/ps` within a second |
| llama.cpp, router mode | `GET /models` (the `/v1/models` handler): every model the router knows, its `--models-dir` and the Hugging Face cache | `POST /models/load`; returns at once, the model listed `loading` until ready | `POST /models/unload`; returns at once |
| llama.cpp, one model | — | — | — |
| vLLM, with `VLLM_SERVER_DEV_MODE=1` | its one model, once `GET /is_sleeping` answers | `POST /wake_up` | `POST /sleep?level=1`: weights to host memory, requests in flight aborted; works only with `--enable-sleep-mode`, which steth does not check |
| geode, with `--models-dir` | `GET /models/available`: each file its `--models-dir` allows, by resolved path, and `GET /v1/models`: each model's `state` and `path` | `POST /models/load` with the file's path; answers 202, the model `loading` until ready | `POST /models/unload` with the model's id; drains it, `unloading` until gone |

geode (from d25f890 on its `steth` branch) lists a split model once, by its
first part with the bytes of all parts, as steth does, and loads the set
from any part's path under the first part's id. Checked in geode on a model
split in eight; not yet through steth's page.

The interface is optional: an adapter that lacks the controls doesn't have
the methods, so a new engine starts read-only. A llama.cpp server of one
model and a vLLM server not in dev mode have the methods but answer
`control.ErrNone`. An engine's refusal reaches the page in its own words
(`model is already running`).

### Launch, from profiles in a file

Monitoring stays zero-config. Launching needs one file, `--config
steth.json`, because executables and model paths are the machine's, and the
browser never supplies a command:

```json
{
  "models": ["C:/models", "D:/more/Qwen3.5-9B-Q4_K_M.gguf"],
  "launch": [
    {
      "name": "geode",
      "command": ["C:/geode/bin/geode.exe", "serve", "{model}", "--models-dir", "{place}", "--port", "8080", "--lanes", "1"],
      "dir": "C:/geode",
      "env": {"HOME": "C:/msys64/home/me"}
    },
    {
      "name": "vllm",
      "command": ["vllm", "serve", "{model}", "--port", "8000"],
      "models": ["Qwen/Qwen3-8B"]
    },
    {
      "name": "llama router",
      "command": ["llama", "serve", "--models-dir", "/models", "--port", "9931"]
    }
  ]
}
```

`{model}` is replaced by the chosen model; a profile without it starts as
written. An argument with `{place}` repeats once for each place, with the
flag before it, so geode starts allowed to open every place's files. A
profile's own `models` are its choices; one that lists none takes the model
files under the places: the file's `models`, a list of directories and
`.gguf` files that are always there, then those added on the page, kept in
`--places` (by default `steth/places.json` in the user's config directory).
A place is an absolute path, there when added; one gone later is kept and
skipped. The page picks a place by listing one directory at a time through
steth, its subdirectories and `.gguf` files only. steth looks through each directory four levels deep, skipping hidden
entries, and lists the `.gguf` files with their size: a split model once, by
its first part, with all its parts' bytes; a vision projector (`mmproj*`)
not at all; a link as the file it points to, as in a Hugging Face cache. It
looks again at every read, so a file copied in shows without a restart, and
lists at most 1,000. Every entry in the file must be there when steth
starts. A start takes only a choice listed now, never a path from
the page. Before a start, steth refuses a profile it already runs,
and one whose command names a port (`--port N`) that already answers on this
machine: the engine is running already, started some other way. It stops
only a process it started, with that process's own children, and stops all
of them when it exits. Each child keeps its last 120 log lines,
each cut at 2,000 characters. A started engine is found by discovery like
any other, so there is one path from process to page.

### Chat, to try a model

The page sends a prompt to one model on a source steth already reads (never
an address from the browser) as an OpenAI chat completion, streamed. steth
passes the stream through, closes the engine's request when the page
cancels, and reports that reply's numbers:

| number | from | fidelity |
|---|---|---|
| first text | seconds from send to the first text, as steth sees it; includes queueing, prefill and transport | measured |
| decode, prefill | geode: `timings.decode_tok_s`, `prefill_tok_s`. llama.cpp: `timings.predicted_per_second`, `prompt_per_second` | measured |
| decode | elsewhere: completion tokens from `usage` (asked for with `stream_options.include_usage`) over the seconds after first text | derived |

These numbers describe one reply steth sent, so they stay in the reply pulse
and never enter `models[].decode_tps`, which covers all traffic. Chats live
in the page only.

### The page

One page in two columns: memory pools (01), models (02) and model files (03) on the left and,
where steth takes actions, the workspace on the right, held in view as the
left scrolls, with the conversation taking the height the rest leaves.
Under 1100 px wide the workspace follows the models; bound off loopback the
box takes the whole width. The toolbar picks one model from every engine's
loaded models and those an engine with controls could load, grouped by
engine and endpoint, with *Load* or *Unload* for it; and, with launch
profiles, an engine profile, its `{model}`, *Start* and *Stop*. A line
under the toolbar, there only when it has something to say, gives what the
last action did or the engine's refusal; the masthead's `LOCAL ONLY` says,
on hover, where prompts go. Below, the conversation (04) with the model picked, one kept per model, and
its pulse (05): the last reply's decode, prefill and first text with where
each came from (`ENGINE`, `DERIVED`), the time this page waited, the
model's decode speed from the state over two minutes, its state, requests,
KV and endpoint, and the picked profile's log. Bound off loopback the
workspace is not drawn and the masthead says `MONITOR ONLY`. Clicking a pool
or a model opens its details in a drawer, as before, with *Talk to it* for
a model. Model files (03), where steth takes actions, shows the
places (those from the config marked, the others removable), *Add a place*
to browse to a directory or `.gguf` file, and every model file under them
with its size. A file's row says what each engine that loads by file does
with it: in it with *Unload*, loading, *Load into* it, or failed with why,
which stays until *Load again* or *Unload* clears it;
where none lists it, *Start* each stopped profile that takes model files
with it; a running one started before the place was added is named.

### Guard

Every action route checks that `Host` is a loopback name (`localhost`,
`127.0.0.0/8`, `::1`), so a site that rebinds its own name to 127.0.0.1 is
refused; a POST also needs `Origin` to be the page's own and a JSON body
under 1 MB with no unknown field. The action GETs carry no CORS header, so
no other site can read them. The page is served with a CSP that runs only
its own script, by hash, and talks only to steth. Bound to a non-loopback
address, steth serves no action routes, and `--config` is refused.
`GET /api/v1/state` is unchanged.

### Routes

| route | body | answer |
|---|---|---|
| `GET /api/v1/controls` | — | `[{engine, endpoint, models, files?, error?}]`: every engine with controls and the models it could load or unload; `files`, from an engine that loads by file (geode), is `[{path, model?, state?, error?}]`, each file it may open and what it is now (`loading`, `ready`, `failed`, `unloading`) |
| `POST /api/v1/load`, `/api/v1/unload` | `{engine, endpoint, model}` | `{model, done}`, done `loaded` or `unloaded`, or `loading` or `unloading` from an engine that answers once begun (geode); 404 for an engine steth does not read or one without controls, 502 with the engine's message when it refuses |
| `POST /api/v1/chat` | `{engine, endpoint, model, messages, max_tokens?, temperature?}` | the engine's event stream as it arrives, then `event: steth` with the reply's numbers, or `event: error` if the stream broke; 404 for a model steth does not read |
| `GET /api/v1/launch` | — | every profile in config order: `{name, models, files, running, model?, pid?, started?, exit?, log}`; `files` is `[{path, bytes}]` for a profile that takes the model files, else null |
| `POST /api/v1/launch/start`, `/stop` | `{name, model?}` | `{name, done}`; 409 with the reason for a refusal; 404 without `--config` |
| `GET /api/v1/library` | — | `{places: [{path, fixed}], files: [{path, bytes, real}]}`: the places and the model files under them; `real` is the path with every link followed, as an engine that resolves links names it |
| `POST /api/v1/library/add`, `/remove` | `{path}` | the library after it; 409 with the reason: not absolute, not there, neither a directory nor a `.gguf` file, a place already, or one from the config file |
| `GET /api/v1/library/browse?dir=` | — | `{dir, parent, dirs, files, cut}`: one directory's subdirectories and `.gguf` files, hidden entries left out; no `dir` is the home directory |

Errors are `{"error": message}`.

## Stages

1–7 are built. geode, Ollama and llama.cpp (two slots, with and without
`--metrics`, idle sleep, router mode) are checked live. Owed: a live check
of vLLM, written from its source (needs a GPU of compute capability 7.0 or
more); discovering a real engine on macOS and Windows, whose box reading CI
checks. geode reports the tokens and seconds its prefill and decode passes
took on `/v1/pool` from its `steth` branch on, checked live here.

1. Skeleton, schema, collector, geode adapter, page ported.
2. Ollama adapter.
3. llama.cpp adapter.
4. Speed, demand and config, through every adapter that has them.
5. Discovery (find engines on local ports) and box layer (pool capacities,
   per-process VRAM and RSS).
6. vLLM adapter.
7. Docs, release binaries, CI.

Managing, each proving one thing:

8. Guard: Host and Origin checks, CSP, no actions off loopback. Tests
   prove a foreign page, a missing Origin, a rebound name, a LAN address and
   an origin on another port get 403 while the state reads as before; live,
   the page runs under its CSP, and steth bound to `0.0.0.0` serves no
   action route and refuses `--config`.
9. Chat: proxy, cancel, per-reply numbers. geode, llama.cpp and Ollama
   streams recorded live in `internal/chat/testdata/`. Checked live through
   the page on Ollama and geode (9.4 tok/s as geode timed it, first text in
   0.97 s); cancelling a reply stopped geode after 19 of 400 tokens.
10. Load and unload: Ollama and llama.cpp router live, Ollama through the
    page too; vLLM from source, accepted without a live check (see below);
    geode once it says how.
11. Launch: profiles, children, logs, refusals. Checked live on Linux: a
    profile whose `--port` already answered was refused; a started geode
    appeared through discovery within 7 s, stopped from the page, and when
    steth got SIGINT the geode it had started went with it. Windows, where
    children stop with `taskkill /T /F`, is accepted without a run (see
    below).

Accepted unchecked on 2026-11-08, by decision: vLLM against a running server
(needs a GPU of compute capability 7.0 or more; this box has 5.0) and
launching on Windows. Both stay listed as the first things to check when a
suitable box is at hand.

## Out of scope

Remote boxes beyond `--target`, authentication, Prometheus output, alerting,
downloading models, changing an engine's settings while it runs, routing or
balancing requests, saving chats, system prompts.
