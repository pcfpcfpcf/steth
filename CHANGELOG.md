# Changelog

Newest first. A release's section is its notes.

## Unreleased

First version. One page for the inference engines on a box.

- Engines: geode (`/v1/pool`), Ollama (`/api/ps`), llama.cpp `llama serve`
  (`/props`, `/slots`, `/metrics`, router mode) and vLLM (`/v1/models`,
  `/metrics`). vLLM is written from its source and not yet checked against
  a running server.
- Finds the engines on its machine by their processes and default ports;
  `--target ENGINE=URL` adds others.
- Reads the machine: each memory pool's size and use, and what local
  llama.cpp and vLLM processes hold, on Linux, macOS and Windows.
- Per model: memory in each pool, KV cache held and its size, state,
  requests running and waiting, decode and prefill speed with two minutes of
  history, unload time, and settings as the engine reports them; every
  number marked measured or derived, a dash where the engine does not say.
- `GET /api/v1/state` serves the same state as JSON (docs/schema.md).
- Manages too, from the page and only on a loopback address: chat with a
  model and see that reply's first-text time and speeds; load and unload
  models on Ollama, a llama.cpp router and vLLM in dev mode; start and stop
  engines from launch profiles in `--config FILE`. Every action goes
  through the engine's own controls; requests from other sites are refused.
- geode's decode and prefill speeds, derived from the totals its `steth`
  branch reports.
- The page in two columns: memory pools, models and model files on the
  left; the conversation and its numbers beside them.
- Model files: the `.gguf` files under the places in the config's `models`
  and those added from the page, which browses this machine one directory
  at a time (`--places` keeps them). A split model is listed once.
- Load and unload a file into a running geode from its row; a failed load
  shows geode's reason and can be loaded again. A row says why no engine
  can open a file, such as a geode started with no files allowed.
- `{place}` in a launch profile repeats per place with its flag, so an
  engine starts allowed to open them.
