`live.*` are recorded from `llama serve` build b10217-ddd4ec142, started
CPU-only with Qwen3.5-0.8B-Q8_0, two slots, an 8192-token cache and
`--metrics`:

- `live.props.json`; `live.slots.json`, one slot holding a finished
  400-token answer.
- `live.slots.gen.1.json`, `live.slots.gen.2.json`: 1.06 s apart while a
  second request generates, 124 then 152 tokens decoded.
- `live.metrics.1.txt`, `live.metrics.2.txt`: after the first request and
  after the second (300 more tokens in 11.193 s of decoding). The counters
  move only when a request ends.
- `sleeping.props.json`: `live.props.json` with `is_sleeping` set.

`router.*` are recorded from the same build started as a router
(`--models-dir` with two models, `--no-models-autoload`, `--metrics`), with
Qwen3.5-0.8B-Q8_0 loaded through it and the rest unloaded; the `child`
files are that model read through `?model=` with `autoload=false`.
`router.states.models.json` is not recorded: it is written from the server
source at commit a469940 (`get_router_models`) for the states not seen
here, a model sleeping and one loading.
