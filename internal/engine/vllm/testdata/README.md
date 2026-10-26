Not recorded from a running server: this box's GPU is too old for vLLM.
Each file is written to the shapes in vLLM's source at commit 442f3d9:
`ModelCard` in `vllm/entrypoints/serve/engine/protocol.py` for `models.json`,
and the metrics `vllm/v1/metrics/loggers.py` registers, as
`prometheus_client` prints them, for the rest.

- `models.json`: one model served, with a LoRA adapter on it.
- `metrics.1.txt`, `metrics.2.txt`: a second apart; three requests running,
  one waiting, 150 tokens generated between them, an eighth of a 65,536-token
  KV cache in use.
- `sleeping.txt`: the engine asleep, its weights offloaded.
