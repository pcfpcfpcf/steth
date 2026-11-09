# TODO

Open work only, most valuable first. steth is the monitor shipped on boxes
that geode's installer sets up and leaves (geode's docs/TODO.md, "Leaving a
box"), so the order follows what such a box needs: right numbers on the
hardware it ships on, an engine that stops shown as stopped, and a version to
pin.

1. **No GPU pool on AMD or Intel.** The box reads GPU memory only from
   `nvidia-smi` (`internal/box/box.go`); without it there is no GPU pool and
   no per-process GPU memory. The box geode is to ship on is a Ryzen AI Max+
   395, AMD with unified memory, and one of geode's three test boxes is an
   RX 6600 XT. On those steth draws host memory only.

   Probable Solution: on Linux, amdgpu and the Intel drivers report the
   card's totals in sysfs (`/sys/class/drm/card*/device/mem_info_vram_total`,
   `mem_info_vram_used`, and `mem_info_gtt_*` for the share of host memory a
   unified-memory chip maps) and each process's use in
   `/proc/<pid>/fdinfo/*` (`drm-memory-vram`, `drm-memory-gtt`). A unified
   box's GTT is host memory, so it must not be counted twice. Windows and
   macOS: not looked into.
2. **An engine that stops disappears.** A discovered engine that no longer
   answers is dropped at the next look, within 10 s
   (`discover.Registry.Refresh`), and its models and history go with it. A
   crashed `geode serve` looks like a box with nothing loaded, not like a
   failure. Only `--target` sources keep showing their error.

   Probable Solution: keep a discovered source that has gone as `down`, with
   when it was last read, for a bounded time or until another engine takes
   its port; the page shows it as stopped. `GET /api/v1/state` carries the
   same, so a probe can alert on it.
3. **geode's speeds and file loads wait on its `steth` branch.** steth
   derives decode and prefill speed from the totals `/v1/pool` reports, and
   loads and unloads files through `/models/*`, both only on geode's
   `steth` branch, not yet in its master. `/v1/pool` still has no
   `"schema"` field.
4. **No release.** CHANGELOG.md has only "Unreleased", and the README marks
   geode's row internal and not out. geode's submodule follows a commit, not
   a version.

   Probable Solution: the README's Release section: a `## v0.1.0` entry and a
   pushed tag, then geode pins the submodule to it.
5. **Nothing starts it with the box.** steth runs in the foreground until
   stopped. On a box left alone it has to start at boot and come back after
   a crash.

   Probable Solution: none written. A systemd unit on Linux and a service on
   Windows, installed by whatever installs geode, not by steth itself.
6. **Checks owed** (docs/design.md, Stages):
   - vLLM against a running server; it needs a GPU of compute capability
     7.0 or more.
   - Discovering a real engine on macOS and Windows; CI checks only the box
     reading there.
   - `nvidia-smi` on Windows.
   - llama.cpp router mode's `sleeping` and `loading` states, written from
     source.
7. **geode drops out while its queue is full.** With every lane and queue
   place taken, geode's one intake thread waits for room and answers
   nothing, `/v1/pool` included, so discovery loses it until the queue
   drains. Work in geode: keep answering the listing routes, or refuse
   with 503, while the queue is full.
8. **A place added on the page reaches a running geode only at its next
   start;** geode takes its allowed files from `--models-dir` flags only.
9. **Known wrong or unread:**
   - vLLM KV usage under data parallelism is summed over engines against
     engine 0's size: wrong past one engine.
   - llama.cpp flags given as `LLAMA_ARG_*` environment variables are not
     seen, so its config and KV layout can be read wrong.
   - Ollama reports no speed or busy state; a timing proxy was rejected
     (LOG.md).

## Out of scope today, asked for by a box left alone

docs/design.md leaves these out: remote boxes beyond `--target`,
authentication, Prometheus output and alerting. A box nobody sits at needs
some of each. Whether they come into steth is undecided; what each would need:

- **Seeing the box from elsewhere.** `--listen` can bind beyond 127.0.0.1,
  but with no authentication anyone on the network then reads every model
  and process command line on the box. Either authentication in steth, or a
  proxy in front of it as geode's docs/TODO.md argues for `serve`.
- **History past two minutes.** "How was it last week" is Prometheus' or
  Grafana's job (docs/design.md); steth would need to export its state in a
  shape they scrape.
- **Alerting.** geode's docs/TODO.md ("Leaving a box", item 4) puts it in a
  probe outside both programs. That probe could read `GET /api/v1/state`
  rather than each engine, once item 2 above makes a stopped engine visible
  there.
