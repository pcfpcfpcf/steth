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
    {"name": "vram", "capacity_bytes": 4294967296},
    {"name": "dram", "capacity_bytes": 16777216000},
    {"name": "nvme", "capacity_bytes": null}
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
      "fidelity": {"kv_tokens": "measured", "context": "measured", "pools": "measured"}
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
| `capacity_bytes` | integer or null | its size; null for a pool with no fixed size (a drive holds what it is given); 0 when the box has none |

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
| `state` | `"serving"`, `"queued"`, `"moving"`, `"idle"` | serving a request; waiting for its turn; being loaded or placed again; loaded and doing nothing. A new state may be added; readers show an unknown one as its raw name |
| `queued` | integer or null | requests waiting for this model |
| `kv_tokens` | integer or null | tokens its KV cache holds now |
| `context` | integer or null | tokens its KV cache is sized for now |
| `max_context` | integer or null | the most tokens it can grow to |
| `pools` | object or null | per pool name: `weight_bytes`, what it holds there besides its KV cache, and `kv_bytes_per_token`, what each token of context reserves there. The KV cache reserved in a pool is `kv_bytes_per_token × context`; in use, `kv_bytes_per_token × min(kv_tokens, context)` |
| `fidelity` | object | field name → `"measured"` or `"derived"` |

## sources[]

One per engine endpoint steth reads.

| field | type | meaning |
|---|---|---|
| `engine` | string | as in `models[]` |
| `endpoint` | string | as in `models[]` |
| `ok` | boolean | whether the last read succeeded |
| `error` | string | what went wrong, when `ok` is false |
