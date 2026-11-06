# PENANDA — Technical Notes & Deferred Work

## Status selesai (done)

- SSE multi-client: 1 producer → broadcast N client (`main.go` hub).
- Zram `used` = `SwapTotal - SwapFree` (`/proc/meminfo`), cocok `free -h`.
- Semua ukuran diseragamkan **byte** di JSON; FE konversi via `fmtB` (1024).
- `zram.go` digabung ke `ram.go`.
- Nama program (card "Program Berjalan"): penuh via `/proc/<pid>/cmdline` (argv0 basename), bukan comm 15-char — cegah merge 2 program beda (`xdg-desktop-por` case).
- RAM per program: **lazy PSS** (`smaps_rollup` 30 kandidat teratas by RSS) — shared dibagi rata, bukan dobel.

## Ditunda (add when butuh)

### 1. Duplikasi parse `/proc/meminfo`
- **Lokasi:** `modules/ram.go` — closure `get` (RAMUsage) vs `memKb` (swapUsed).
- **Masalah:** pola sama `Key: <n> kB`; file dibaca 2× per tick.
- **Kenapa ditunda:** bukan bug, cost tak terukur di 1 tick/s.
- **Kapan dibereskan:** N client besar / collect > 1s budget / tick mulai macet.
- **Cara:** satu helper `meminfoVal(path) map`, satu baca untuk RAM+zram/swap sekaligus.

### 2. Minor lain (bukan prioritas)
- `splitLines` (log.go) ≈ `strings.Split(s, "\n")` + skip kosong.
- Pola `os.ReadFile` + `strings.Fields` berulang (server/cpu/network).

## Unit data (bytes)

| Field | Unit vs sumber |
|---|---|
| RAM total/avail/used | byte (`/proc/meminfo` kB × 1024) |
| Storage | byte (statfs) |
| Zram disk/used/compressed | byte (`used` = meminfo swap) |
| Network rx/tx | byte (counter) |
| Rate (mbps), %, uptime, load | bukan byte |