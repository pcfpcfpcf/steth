# Log

Decisions as they are made, newest first.

## 2026-10-26

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
