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

`router.*` are not recorded: they are written field for field from the
server source at commit a469940 (`get_router_props`, `get_router_models`),
for `llama serve` started without a model, managing several: one loaded,
one sleeping, one loading, one unloaded.
