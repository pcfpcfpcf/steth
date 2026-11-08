Real answers to `POST /v1/chat/completions`, recorded with `curl -sN` on
2026-11-08, prompt "Say hello in five words.", `max_tokens` 20, `stream`
true, `stream_options.include_usage` true.

| file | engine | what it shows |
|---|---|---|
| `geode.sse` | geode (built from the local tree; it reports no version), model Qwen3.5-0.8B | `timings.prefill_tok_s` and `decode_tok_s` on the `finish_reason` chunk; no `usage` chunk although asked for; a stray NUL byte after the last event |
| `llamacpp.sse` | llama.cpp `llama serve` b10217-ddd4ec142, Qwen3.5-0.8B-Q8_0.gguf, CPU only (`-ngl 0`) | text starts as `reasoning_content`; reply cut at 20 tokens (`length`); `usage` and `timings` (`predicted_per_second`, `prompt_per_second`) on one last chunk with empty `choices` |
| `ollama.sse` | Ollama 0.21.2, qwen2.5:1.5b | no timings; `usage` on a last chunk with empty `choices`, sent only when `include_usage` is asked for |
| `geode-refused.json` | geode, model "nope" | the 400 body |
| `ollama-refused.json` | Ollama 0.21.2 | the 500 body when its runner crashed loading a model |

The Ollama stream came from a second `ollama serve` on CPU on a spare
port, reading the system service's models: the service's own runner
crashed (`ollama-refused.json`) with the GPU full.
