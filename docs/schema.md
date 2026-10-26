# State schema

`GET /api/v1/state` returns one JSON object: every pool on the box, every
model every engine has loaded, and whether each engine answered. The page
reads only this; any other tool can too.

Fields are added freely and never renamed. `schema` changes only when a
field's meaning does.

## Example

```json
{
  "schema": 1,
  "pools": [
    {"name": "vram", "capacity_bytes": 4294967296, "used_bytes": 3221225472},
    {"name": "dram", "capacity_bytes": 16777216000, "used_bytes": 6442450944},
    {"name": "nvme", "capacity_bytes": null, "used_bytes": null}
  ],
  "models": [
    {
      "id": "qwen3-4b",
      "engine": "geode",
      "endpoint": "http://127.0.0.1:8080",
      "state": "serving",
      "queued": 1,
      "kv_tokens": 812,
      "context": 4096,
      "max_context": 32768,
      "pools": {
        "vram": {"weight_bytes": 2147483648, "kv_bytes_per_token": 73728},
        "dram": {"weight_bytes": 0, "kv_bytes_per_token": 0}
      },
      "running": 1,
      "decode_tps": 14.2,
      "prefill_tps": null,
      "unloads_at": null,
      "config": null,
      "fidelity": {"kv_tokens": "measured", "context": "measured", "pools": "measured", "decode_tps": "derived"}
    }
  ],
  "sources": [
    {"engine": "geode", "endpoint": "http://127.0.0.1:8080", "ok": true}
  ]
}
```

## pools[]

| field | type | meaning |
|---|---|---|
| `name` | `"vram"`, `"dram"`, `"nvme"` | the memory: the GPU's, the host's, a drive weights stream from |
| `capacity_bytes` | integer or null | its size; null where no size is known (a drive holds what it is given; an engine may not say); 0 when the box has none |
| `used_bytes` | integer or null | what is in use by anything, models and every other process, as the operating system says; null where it does not. The first source to know it sets it |

## models[]

A number is null when the engine does not say it. `fidelity` names how each
non-null field was obtained: `"measured"` (the engine or the OS reports it)
or `"derived"` (steth computes it from what is reported, such as a rate from
two counters). A field absent from `fidelity` is either null or always
known (`id`, `engine`, `endpoint`, `state`).

| field | type | meaning |
|---|---|---|
| `id` | string | the model's name as its engine gives it |
| `engine` | string | `geode`, `ollama`, `llamacpp` or `vllm` |
| `endpoint` | string | the engine's base URL |
| `state` | `"serving"`, `"queued"`, `"moving"`, `"idle"`, `"loaded"`, `"sleeping"` | serving a request; waiting for its turn; being loaded or placed again; loaded and doing nothing; loaded, its engine not saying whether it is busy; its memory freed while idle, to be loaded again on the next request. A new state may be added; readers show an unknown one as its raw name |
| `queued` | integer or null | requests waiting for this model |
| `kv_tokens` | integer or null | tokens its KV cache holds now |
| `context` | integer or null | tokens its KV cache is sized for now |
| `max_context` | integer or null | the most tokens it can grow to |
| `pools` | object or null | per pool name: `weight_bytes`, what it holds there besides its KV cache, and `kv_bytes_per_token`, what each token of context reserves there. The KV cache reserved in a pool is `kv_bytes_per_token × context`; in use, `kv_bytes_per_token × min(kv_tokens, context)`. `kv_bytes_per_token` is null where the engine does not split its KV cache from the rest; `weight_bytes` then includes it |
| `running` | integer or null | requests it is serving now |
| `decode_tps` | number or null | tokens per second it generated over the last poll, while generating |
| `prefill_tps` | number or null | prompt tokens per second it processed over the last poll, while processing |
| `unloads_at` | string or null | RFC 3339 time its engine will unload it if no request comes first |
| `config` | object or null | setting name → `{"value", "source"}`: how the engine was launched or the model loaded, as the engine reports it (`source` names where: an endpoint, or `launch args`). Names are the engine's own, read-only |
| `fidelity` | object | field name → `"measured"` or `"derived"` |
| `speed_history` | array | the polls of the last two minutes that had a speed, oldest first: `{"time", "decode_tps", "prefill_tps"}`. Empty for an engine that reports no speed |

## sources[]

One per engine endpoint steth reads.

| field | type | meaning |
|---|---|---|
| `engine` | string | as in `models[]` |
| `endpoint` | string | as in `models[]` |
| `ok` | boolean | whether the last read succeeded |
| `error` | string | what went wrong, when `ok` is false |
