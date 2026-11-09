# Log

Decisions as they are made, newest first.

## 2026-11-09

- **steth has a profile of its own for each engine found** that no config
  profile starts, so the page starts an engine with no config: before,
  with no `--config`, it could start nothing. Chosen over generating a
  config file: these change with what is installed, and the config file
  stays the person's. A config profile for an engine (its program's file
  name, or the very path) replaces steth's, and keeps its name if it is
  the engine's. geode's starts with no model or with a file, on a quiet
  port from 8080; Ollama's is `ollama serve`, refused while 11434 answers
  (the system service here); llama.cpp's serves the one file it is given,
  from 9931 (a router would need a models directory steth would choose);
  vLLM's takes a file from the places, which vLLM reads only
  experimentally, unchecked as no vLLM runs here. Programs are found at
  start-up only: one installed later needs a restart, rather than asking
  versions again every few seconds. A config profile's program names its
  engine's, so a geode reached only through the config is installed too.
  `--engine`, like `--config`, is refused off loopback. Checked live:
  geode named with `--engine`, Ollama 0.21.2 and llama.cpp b10217 found on
  PATH, each with a profile.

- **An engine is installed when its program is found,** asked for by the
  owner, as discovery sees only engines that listen. Each adapter names
  its programs (`geode`; `ollama`; `llama`, then `llama-server`; `vllm`);
  steth takes the one named for the engine, else the first on PATH that is
  the engine's own. A program is asked its version only where that is
  cheap and safe: `ollama --version` and llama.cpp's `--version` answer at
  once. geode has no version flag and opens `--version` as a model file
  (it starts CUDA first), and `vllm --version` imports torch, so neither
  is run. `llama` is a common name, so it counts only when it prints a
  llama.cpp build (`b10217-ddd4ec142`); `llama-server` is llama.cpp's own
  name. Each answer gets 2 s and its first 4 KB. A relative directory on
  PATH is skipped, as a program from the current directory would be. A
  named program is trusted, whatever its file name.
- **`~/.local/bin/llama` on this box is llama.cpp,** build b10217
  (ddd4ec142): `llama --help` lists `serve`, `cli`, `update`, `download`,
  and `llama serve --version` prints llama.cpp's `version: 10217
  (ddd4ec142)`, `built with Clang 22.1.8`. It is the build the llama.cpp
  adapter was checked against, so steth lists it as installed.

- **A profile's model may be optional** (`model_optional`). geode serves
  with no model and stays up, then loads files from the page (from the
  session working on geode), so one geode profile both starts empty and
  starts with a file from its row. Leaving `{model}` out takes the flag
  before it, as `{place}` with no place does, so `-m {model}` goes whole.
  Opt-in, as a llama.cpp server with no `-m` is a router, not the same
  server without a model. The toolbar offers *no model* first for such a
  profile.

- **`{port}` picks a quiet port at each start,** from the profile's `port`
  up, 100 at most. A profile that starts an engine wherever this box has
  room needs it: the owner's geode already holds 8080 and 8082, and a
  fixed `--port` would be refused. Asking the OS for a free port (`:0`)
  was rejected: it lands far from the engine's own default, which is where
  people look. A `port` without `{port}` is where an engine listens that
  takes no port flag (`ollama serve`), checked quiet like `--port`. Two
  starts at once may pick the same port before either binds; the second
  engine then fails and its log says so.

- **A file row says why a running geode cannot open it,** rather than
  *No engine can open it*: geode was started without that file allowed,
  or with no model files allowed. Checked live against geode (branch
  `steth`) on port 8094: started with no `--models-dir`, every row said
  so; started allowing the 0.8B file, its row offered *Load into geode
  :8094*, loaded (loading, then ready) and unloaded through steth, and
  the rest said geode was started without them. geode was discovered
  within one discovery round (10 s) whether started with a model or
  none, and its `/v1/pool` answered within 1 ms with 12 requests in
  flight (4 lanes, 8 queued).
- **A geode with a full queue looks gone.** With 16 requests in flight
  geode's one intake thread waits for room in its queue of 8, so nothing
  more is accepted: 29 of 30 `/v1/pool` reads timed out at 500 ms, and a
  discovery round then drops geode until the queue drains. Fixed only on
  geode's side, by answering the listings while the queue is full.

- **A geode that may open no file still lists its files,** as `[]`. The
  owner saw *No engine can open it* on every file row with geode running:
  `/api/v1/controls` dropped an empty `files` with `omitempty`, so a geode
  started without `--models-dir` looked like an engine that loads by name.
  `omitzero` keeps the empty list and still leaves `files` out for Ollama,
  llama.cpp and vLLM.

## 2026-11-08

- **A load that has only begun says so.** geode answers a load or unload
  once begun, so steth's reply says `loading` or `unloading` for it, not
  `loaded`; the page already waits on the model's state.

- **A split model loads by its first part.** geode lists each set once by
  its first part, with all parts' bytes, and loads it from any part's
  path; checked in geode on a model split in eight (geode d25f890), not
  yet through steth's page. steth needed no change.

- **Load and unload by file checked live** against geode's routes (geode
  branch `steth`, d33c40f) on port 8091: geode started from a file's row
  with `--models-dir` from the place (up in 3 s); the 4B loaded from its
  row, loading then ready in about 3 s; a chat with it (decode 6.6 tok/s,
  prefill 11.8 tok/s, both from geode); a broken 19 KB file failed with
  geode's reason and its row offered *Load again* and *Unload*; both
  unloaded from the page, and geode's saved set for the port left empty.
  The page reads the controls every second while a file loads or
  unloads, as a row lagged up to five seconds.

- **A failed load stays until the user clears it,** as geode keeps the
  failed slot and its error: a load of the same path replaces it, an
  unload of its id frees it. The file's row offers both.

- **Places of model files picked on the page,** asked for by the owner,
  built beside geode's runtime loads (agreed with the session working on
  geode). *Model files* browses this machine one directory at a time
  through steth, directories and `.gguf` files only, and adds a directory
  or file as a place; places added there are kept in a file of steth's own
  (`--places`), apart from the config's, which the page cannot remove. A
  place is any absolute path the page sends, but it only names where to
  look: a start or load still takes only a file steth or the engine lists.
  `{place}` in a profile repeats per place with its flag, so geode starts
  allowed to open them; geode keeps its allow-list from flags only, so a
  place added later reaches it at its next start, and the row says so.
  Checked live: a place added from the page and kept across restarts,
  geode started from a file's row with `--models-dir` from the place, the
  served file shown in it.
- **geode loads and unloads by file,** to the contract agreed with the
  session working on geode: `/models/available` lists the files its
  `--models-dir` allows, `/models/load` takes a path and answers 202,
  `/v1/models` gives each model's `state` and `path`, `/models/unload` takes
  the id. An engine that loads by file lists its files, each with what it
  is now, in `/api/v1/controls`. A geode whose `/v1/models` names no path
  yet still marks a file it serves, through `served`. Written against the
  contract before geode has the load routes; checked live only that far
  (its files listed, the served one marked; a load answers 404).

- **Model files come from places in the config,** not only from each
  profile's list. Asked for by the owner: a model should be added by its
  directory or file. A top-level `models` lists directories and `.gguf`
  files; a `{model}` profile that lists none takes every model file under
  them, found again at each look (30 ms for the owner's 10 files, 150 GB),
  so a copied-in file shows without a restart. The page still never sends
  a path steth did not list. A split model is one entry with all its
  parts' bytes; `mmproj` files are left out, being no model on their own.
  Profiles keep their own `models` for engines that take names, not files
  (vLLM). Loading a file into an engine already running waits on that
  engine having a load route for paths; Ollama is left out, as it needs a
  model made from the file first. Checked live: geode started from a
  scanned file, a path not listed refused, then stopped.

- **The page in two columns.** Asked for by the owner, who did not like
  the workspace stacked under the box. The pools and models go left; the
  toolbar, conversation and pulse go right, sticky and the viewport's
  height, so the conversation and the numbers stay in view while the box
  scrolls, and a reply is watched next to where the model sits. To fit a 900 px tall window the pulse is a strip: its chart is
  shorter, its engine details two columns, its caption a hover. The fixed
  line about where prompts go moved to the hover of `LOCAL ONLY`. Checked
  at 1440, 1280, 1024 and 390 px wide, light and dark, and monitor-only.

- **steth will manage engines too** (docs/design.md, stages 8–11; approved,
  and built the same day): launch engines, chat, per-reply timings, load
  and unload. This reverses "not a model manager". Objection raised and
  overruled: a manager is a bigger product than a monitor, and the controls
  are not uniform across engines. What keeps it neutral: load and unload go
  through each engine's own HTTP controls, shown only where they exist;
  launch runs argv from a config file, never from the browser; chat speaks
  the OpenAI API every engine has. Still rejected: a proxy in other clients'
  path (the Ollama speed decision stands); per-reply chat speeds feeding
  `decode_tps`, since they describe one request steth sent. The page and its
  placement view stay as they are.
- **Load and unload checked live on Ollama and a llama.cpp router** (stage
  10, adapters only; no route or button yet). Ollama 0.21.2 listed its nine
  models; a load of gemma3:1b returned in 2.8 s with the model in memory,
  an unload in 2 ms with the model gone from `/api/ps` within a second. On
  a GPU nearly filled by other processes a first load failed with Ollama's
  own 500, `llama runner process has terminated`, which steth passes on as
  it does every engine's refusal. The router (b10217, `--models-dir`,
  `--no-models-autoload`) answers both at once with `{"success":true}` and
  then lists the model `loading`, or still `loaded`, until it is done;
  loading a running model is a 400 `model is already running`, unloading a
  stopped one `model is not running`, an unknown name a 404
  `File Not Found`. `/models` and `/v1/models` are one handler, and list
  models from the Hugging Face cache beside `--models-dir`. A server of one
  model answers `control.ErrNone` and is sent nothing. vLLM is written from
  its source at commit 442f3d9, its live check owed with the rest: the sleep
  routes need `VLLM_SERVER_DEV_MODE=1` alone, and `--enable-sleep-mode`
  only for sleeping to work, so a dev-mode server without it shows controls
  that fail. Only `/server_info` says whether it is on, and it gathers the
  whole environment (`collect_env`) to answer, so steth does not ask. `/sleep` aborts requests in flight
  by default, as a llama.cpp unload does. The design's table said vLLM's
  routes needed both settings; corrected.

- **geode sums its passes; steth derives the speeds.** On geode's `steth`
  branch `/v1/pool` gives each model `prefill_tokens`, `prefill_seconds`,
  `decode_tokens` and `decode_seconds`, totals since it started, rather than
  a rate over a window geode would have to choose. steth divides the change
  between two reads and marks the speed `derived`; live it read 11.08 tok/s
  where geode's own stream said 11.1. A model idle between reads gets no
  speed rather than zero. The geode submodule in geode's own repo is not
  bumped.
- **The guard.** Actions exist only on a loopback `--listen`; there every
  action route needs a loopback Host (against DNS rebinding) and a POST
  also needs Origin equal to `http://` plus that Host, a JSON body under
  1 MB, and no unknown fields. The page's CSP names its one inline script
  by hash, so nothing injected runs. `/api/v1/state` is unchanged, `ACAO *`
  included: reading was always open to any tool. Off loopback `--config` is
  an error, not ignored, so nobody believes they can launch from there.
- **Chat.** geode sent no `usage` and ended `[DONE]` with a NUL; both fixed
  on its `steth` branch, the recorded stream is from before. Ollama honours
  `stream_options.include_usage`; llama.cpp puts `timings` on the usage
  chunk. Reasoning text counts as first text, since the model has started
  answering. The engine's own timings are used where it gives them (geode,
  llama.cpp); otherwise decode is (completion tokens − 1) over the time from
  first text to the end, marked `derived`.
- **Actions ignore the read client's timeout.** Discovered engines are read
  with a 500 ms client; the first load through the page failed on it. A
  POST copies the client without its timeout and is bounded by its context
  (five minutes for a load), which a test holds.
- **Launch.** A stopped engine's last run, exit and log stay on the page
  until it is started again. A profile is refused if it runs or its
  `--port` answers, so steth never fights another server for a port; a
  child gets its own process group and SIGINT, SIGKILL after 5 s, on
  Windows `taskkill /T /F` (not yet run there). steth stops every engine it
  started when it exits, also when its listener fails.
- **enginetest records POSTs**, so the control tests check what was sent
  against the same recorded answers the read tests use.

- **Stages 10 and 11 accepted without two live checks.** vLLM's load and
  unload have not run against a server: the CPU build of vLLM may not
  support sleeping, and the GPU build needs compute capability 7.0, the box
  has 5.0. Launching has not run on Windows; no Windows machine was at
  hand. Both are written and unit-tested; the owner accepted them as they
  are. Owed first when a box exists.

- **The page redone around a workspace.** Asked for by the owner. The
  pools and models stay on top, unchanged in what they show; the look moves
  to paper and ink with a serif for figures and mono for labels, with a dark
  variant. The Engines cards and the chat inside the details drawer are
  replaced by one toolbar (model, load or unload, engine profile, start or
  stop), the conversation, and a pulse beside it. Chosen over keeping chat
  per model in the drawer: a conversation needs room, and its numbers read
  better next to the model's own speed. The pulse charts the model's decode
  speed from the state rather than streamed events per second, which would
  be a new number that is not a token rate. Generation settings are empty
  by default, so the engine's own apply unless the person sets them. Checked
  live: Ollama load, chat (decode 21.7 tok/s derived) and unload; a geode
  profile started, talked to (9.5 tok/s and prefill 31.5 tok/s, both from
  geode), and stopped; dark mode; 390 px wide with no sideways scroll.

## 2026-10-26

- **macOS and Windows checked in CI** (run 40380470607): the tests that read
  the machine passed on both. Each found its own listening socket with its
  pid (`lsof`, `netstat`), its own memory, command line (`ps`, PowerShell)
  and parent, and host memory. Windows first failed on line endings: its
  checkout turned the recorded fixtures to CRLF, so `.gitattributes` keeps
  `testdata` byte for byte. Not checked there: discovering a real engine,
  and nvidia-smi.
- **macOS and Windows read the box too,** unverified on those systems.
  macOS runs `lsof -F pn` (sockets of this user's processes, or everyone's
  as root), `ps` for command lines, parents and memory, and `sysctl` and
  `vm_stat` (active + wired + compressed pages, as Activity Monitor counts
  memory used). Windows calls the API through `syscall` (host memory, each
  process's working set, the process table: fast enough to run every poll),
  `netstat -ano` for listeners with their PIDs, and PowerShell for command
  lines, which takes most of a second and so runs only at discovery. A
  netstat listener is known by its all-zero foreign address, since the state
  word is translated. The parsers are tested here against outputs in each
  tool's documented format; the tests that read the machine itself run in
  CI on Linux, macOS and Windows once the repository is pushed.
- **Router mode checked live** (`--models-dir`, `--no-models-autoload`).
  `/props` says `role: router`; `/v1/models` lists each model with a status
  and the command line its server runs with. A model not loaded lists
  `--port 0`; once loaded, its real port, so the adapter finds the child
  process by it. steth read the loaded model through `?model=` with its
  child's memory, left the unloaded ones out, and discovery skipped the
  child's own port. Sleeping and loading states were not seen here; their
  fixture stays written from source.
- **Sleep handling checked live** (`--sleep-idle-seconds 5`, no
  `--metrics`). Polling `/slots` every second kept the server awake for 12 s;
  it slept within 7 s of stopping. steth, which skips `/slots` for a server
  launched to sleep, let it fall asleep while polling, showed it `sleeping`
  at 212 MB, `loaded` at 1.1 GB when a request woke it (no `/metrics` and no
  `/slots`: busy state unknown), and `sleeping` again about 6 s after.
- **llama.cpp checked live** (`llama serve` b10217, two slots, with
  `--metrics`). Every field and metric name read from source is there. Its
  default port in this build is 8080. With `-np 2` the server reports
  `kv_unified = false` and 4096 tokens per slot, as steth derives. A slot's
  `n_prompt_tokens` counts generated tokens too, so KV held is right. But
  the token counters in `/metrics` move only when a request ends, so a
  speed derived from them appears only on the poll a request finishes. The
  live decode speed now comes from each busy slot's `n_decoded` growth
  between reads (26.4 tok/s live against 26.8 from the counters afterwards);
  the counters stay the fallback, and still give prefill.
- **Owed after the first build.** Live checks of llama.cpp and vLLM (see
  their entries); the box on macOS (sysctl, IOKit) and Windows; geode
  reporting decode and prefill rates on `/v1/pool` and a `"schema"` field,
  in geode's own repository; vLLM KV usage under data parallelism.
- **Release from a tag.** `scripts/release.sh` builds six targets with
  `CGO_ENABLED=0` and refuses a version with no CHANGELOG section; the
  release workflow runs it and publishes with that section as the notes.
  No release tool (goreleaser) for a dozen lines of shell.
- **vLLM is written from its source at commit 442f3d9;** this GPU (compute
  capability 5.0) cannot run it, so the live check is owed. The plan's
  metric names hold: `num_requests_running`/`waiting`,
  `kv_cache_usage_perc`, `prompt_tokens_total`, `generation_tokens_total`,
  all labelled `model_name` and `engine`. `cache_config_info` is a gauge at
  1 whose labels are every cache setting, including `kv_cache_size_tokens`;
  it carries no `model_name`.
- **vLLM's context is its KV cache's size in tokens** (`kv_cache_size_tokens`,
  else blocks × block size), shared by every request; KV held is usage ×
  size, marked derived. `max_model_len` caps one request, so it is config,
  not `max_context`. With data parallelism, usage is summed over engines
  against engine 0's size: wrong past one engine, left until someone runs
  one.
- **vLLM speeds are tokens over wall time** between two reads: vLLM keeps
  no counter of time spent generating. Under batching that is the server's
  throughput, not one request's speed.
- **A process's memory includes its children:** vLLM keeps its weights and
  cache in an engine-core child process, and its workers under tensor
  parallelism.
- **llama.cpp's own process, where steth can see it,** fills what HTTP does
  not: its memory per pool (nvidia-smi per PID, resident host memory), its
  command line as config, whether it sleeps when idle (then `/slots` is not
  read, so steth no longer keeps it awake; KV held is then unknown), and
  whether its slots split the KV cache (then context is per-slot `n_ctx` ×
  slots, marked derived). Flags given as `LLAMA_ARG_*` environment
  variables are not seen. A port whose process cannot be seen is looked up
  again only every tenth read: finding out scans every process.
- **Discovery is on by default.** A listening socket counts when its
  process runs an engine and a probe of the port answers as that engine; a
  socket of another user's process (Ollama runs as its own service user) is
  probed only on the engines' default ports, so steth never sends HTTP to an
  unknown service. A llama.cpp server started by a llama.cpp router is
  skipped: the router reports it. Each found engine keeps its adapter across
  rounds, so derived speeds survive rediscovery.
- **The box is a source,** read first so its measured pool sizes win over
  an engine's own view, and it reports what is in use by anything; the page
  draws that remainder as "Other". A box without nvidia-smi reports no GPU
  pool rather than failing. Only Linux is read so far.
- **Speeds are the engine's, or none.** The ported page guessed a rate from
  KV cache growth; that counts a prompt's prefill as decoding, so it is
  gone. geode shows no speed until it reports one itself. llama.cpp speeds
  are derived from counter deltas over the seconds spent on them.
- **The collector keeps two minutes of speeds** per model (at most 240
  samples), only polls that had one, so a page opened late still draws a
  sparkline. Decode and prefill are separate sparklines with their own
  scale: they differ a hundredfold, and one chart has one axis.
- **Config is the engine's own names,** read-only, each with where it was
  read; steth does not translate them into a common vocabulary.
- **llama.cpp is now `llama serve`,** a subcommand of one `llama` binary
  linking the same server code; its default port is 9931, not 8080. The
  adapter is written from the source at commit a469940, not from a running
  server: building downloaded code was not permitted in this session. Live
  check still owed.
- **What llama.cpp says, read from its source.** `/props` (model alias,
  per-slot `n_ctx`, `total_slots`, `endpoint_slots`, `is_sleeping`) and
  `/metrics` never wake a sleeping server. `/slots` does, and resets the
  idle timer, so the adapter skips it while the server sleeps. Polling it
  every second keeps a server started with `--sleep-idle-seconds` from ever
  sleeping; the fix waits for launch args (stage 5) to show the flag.
  `/metrics` has no KV metric at this commit: the `kv_cache_usage_ratio`
  name in the plan belongs to a fork.
- **KV held is the sum of every slot's `n_prompt_tokens`;** a slot keeps its
  last conversation cached after it ends. Context is one slot's `n_ctx`:
  the whole cache in the default layout (four slots sharing one unified
  cache), a 1/N share with `--parallel N` and no `--kv-unified`. HTTP does
  not say which; launch args will.
- **Router mode** (`llama serve` without a model): `/v1/models` lists models
  with a status; each `loaded` one is read through `?model=` with
  `autoload=false`, since a plain GET would make the router load it.
  `sleeping` and `loading` models are listed without reading them; others
  are not in memory and are left out.
- **Ollama speed stays unavailable.** `/api/ps` reports no throughput or
  busy state. A pass-through proxy that times requests was rejected: steth
  reads engines, it does not sit in their path, and a proxy only sees
  traffic sent through it. Revisit if users ask.
- **Ollama models are `loaded`, not `idle`.** Ollama does not say whether a
  model is busy; `idle` would be a guess. The page says "nothing being
  served" only when every engine reports busy state.
- **Null means "not said".** `capacity_bytes` null is a pool whose size no
  source knows (Ollama never says; the box layer will); the first source
  that knows one sets it. `kv_bytes_per_token` null is a model whose engine
  does not split its KV cache from the rest; `weight_bytes` then holds both.
  The page shows a dash, never a zero, for either.
- **A model is keyed by endpoint and name** on the page, so engines that
  load and unload keep each model's colour and drop what was drawn for it.
- **Pools Ollama reports are only those a model is in,** so a box without a
  GPU shows no GPU square.
- **Go, Apache-2.0, module `github.com/pcfpcfpcf/steth`.** One static binary
  with the page embedded; runs on Linux, macOS and Windows. Python (as
  inferwatch) needs a runtime on the box; C needs a hand-rolled HTTP server.
- **Fresh history.** The page comes from a closed engine's dashboard and is
  rewritten engine-neutral; no source or history from that project.
- **Sources named explicitly first** (`--target engine=URL`). Finding engines
  on local ports comes at stage 5; until then the user names them.
