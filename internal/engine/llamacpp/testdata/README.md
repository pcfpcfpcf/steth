Not recorded from a running server. Each file is written field for field
from the serializers in llama.cpp `tools/server` at commit a469940
(`get_res_props`, `server_slot::to_json`, `get_router_props`,
`get_router_models`), which `llama serve` runs. Replace them with recorded
responses once a server can be run here.

- `single.*`: one model, the default four slots sharing one KV cache; slot 0
  processing a request, slot 1 holding a finished one, slots 2-3 unused.
- `sleeping.props.json`: the same server asleep after `--sleep-idle-seconds`.
- `router.*`: `llama serve` started without a model, managing several: one
  loaded, one sleeping, one loading, one unloaded.
- `metrics.*`: the single server started with `--metrics`; two scrapes a
  second apart: 512 prompt tokens in 0.256 s, 24 generated in 0.75 s, two
  requests deferred.
