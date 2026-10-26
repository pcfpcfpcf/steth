`gpus.txt`, `meminfo.txt` and `tcp.txt` are recorded on a laptop with one
Quadro M1200 (4 GB). `apps.txt` and `two-gpus.txt` are written in the same
format as `nvidia-smi --query-compute-apps=pid,used_memory` and
`--query-gpu=memory.total,memory.used` with `--format=csv,noheader,nounits`.

`lsof.txt` and `vm_stat.txt` are not recorded: they are written in the
formats `lsof -F pn` and macOS's `vm_stat` print, for the macOS parsers.
`netstat.txt` is written in the format of Windows' `netstat -ano`, with one
state in another language, for the Windows parser.
