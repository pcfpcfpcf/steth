# Log

Decisions as they are made, newest first.

## 2026-10-26

- **Go, Apache-2.0, module `github.com/pcfpcfpcf/steth`.** One static binary
  with the page embedded; runs on Linux, macOS and Windows. Python (as
  inferwatch) needs a runtime on the box; C needs a hand-rolled HTTP server.
- **Fresh history.** The page comes from a closed engine's dashboard and is
  rewritten engine-neutral; no source or history from that project.
- **Sources named explicitly first** (`--target engine=URL`). Finding engines
  on local ports comes at stage 5; until then the user names them.
