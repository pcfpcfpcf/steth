# Log

Decisions as they are made, newest first.

## 2026-10-26

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
