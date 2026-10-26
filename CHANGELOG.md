# Changelog

Newest first. A release's section is its notes.

## Unreleased

First version. One page for the inference engines on a box.

- Engines: geode (`/v1/pool`), Ollama (`/api/ps`), llama.cpp `llama serve`
  (`/props`, `/slots`, `/metrics`, router mode) and vLLM (`/v1/models`,
  `/metrics`). llama.cpp and vLLM are written from their source and not yet
  checked against running servers.
- Finds the engines on its machine by their processes and default ports;
  `--target ENGINE=URL` adds others.
- Reads the machine on Linux: each memory pool's size and use, and what
  local llama.cpp and vLLM processes hold.
- Per model: memory in each pool, KV cache held and its size, state,
  requests running and waiting, decode and prefill speed with two minutes of
  history, unload time, and settings as the engine reports them; every
  number marked measured or derived, a dash where the engine does not say.
- `GET /api/v1/state` serves the same state as JSON (docs/schema.md).
