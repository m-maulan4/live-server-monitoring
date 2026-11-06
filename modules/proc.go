package modules

import (
	"bytes"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Process mewakili satu program berjalan.
type Process struct {
	PID  int     `json:"pid"`
	Name string  `json:"name"`
	CPU  float64 `json:"cpu"` // persen per proses (delta tiap tick)
	RAM  uint64  `json:"ram"` // byte PSS (footprint unik, shared dibagi)
}

type procSample struct {
	utime uint64
	stime uint64
	t     time.Time
}

const procClkTick = 100.0 // Linux CLK_TCK standar (Hz)

var (
	procMu   sync.Mutex
	procPrev = make(map[int]procSample)
)

// Processes membaca /proc dan mengembalikan `top` program teratas by RAM (PSS).
// Instans program sama dijumlahkan (RAM+CPU agregat, PID = instans pertama).
// RAM: 30 kandidat teratas by RSS → PSS (smaps_rollup, shared dibagi rata);
// sisanya RSS (tidak tampil, cukup ranking).
// CPU% per proses dari delta utime+stime antar panggilan; panggilan pertama selalu 0.
func Processes(top int) ([]Process, error) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, err
	}
	now := time.Now()

	type pinfo struct {
		pid  int
		name string
		rss  uint64
		cpu  float64
	}
	var ps []pinfo

	procMu.Lock()
	defer procMu.Unlock()
	newPrev := make(map[int]procSample)
	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}
		name, rss, utime, stime, ok := procStat(pid)
		if !ok {
			continue
		}
		s := procSample{utime: utime, stime: stime, t: now}
		newPrev[pid] = s

		cpu := 0.0
		if p, ok := procPrev[pid]; ok {
			dt := now.Sub(p.t).Seconds()
			if dt > 0 {
				delta := (float64(utime-p.utime) + float64(stime-p.stime)) / procClkTick
				cpu = delta / dt * 100
			}
		}
		ps = append(ps, pinfo{pid: pid, name: name, rss: rss, cpu: cpu})
	}
	procPrev = newPrev

	// pass 2: PSS cuma 30 kandidat teratas by RSS; lainnya tetap RSS.
	const pssCandidates = 30 // jangkau top-10 tampil + margin
	sort.Slice(ps, func(i, j int) bool { return ps[i].rss > ps[j].rss })
	for i := 0; i < len(ps) && i < pssCandidates; i++ {
		if p := procPSS(ps[i].pid); p > 0 {
			ps[i].rss = p
		}
	}

	// agregat per nama (sum), PID = instans pertama.
	var rows []Process
	seen := make(map[string]int)
	for i := range ps {
		p := ps[i]
		if idx, ok := seen[p.name]; ok {
			rows[idx].CPU += p.cpu
			rows[idx].RAM += p.rss
		} else {
			seen[p.name] = len(rows)
			rows = append(rows, Process{PID: p.pid, Name: p.name, CPU: p.cpu, RAM: p.rss})
		}
	}

	sort.Slice(rows, func(i, j int) bool { return rows[i].RAM > rows[j].RAM })
	if top > 0 && len(rows) > top {
		rows = rows[:top]
	}
	return rows, nil
}

// procPSS: PSS (proportional shared size) dari /proc/<pid>/smaps_rollup, byte.
// Pss: <kB> — halaman shared dibagi rata ke semua pemakai.
func procPSS(pid int) uint64 {
	b, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/smaps_rollup")
	if err != nil {
		return 0
	}
	for _, line := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(line, "Pss:") {
			n, _ := strconv.ParseUint(strings.Fields(line)[1], 10, 64)
			return n * 1024
		}
	}
	return 0
}

// procStat: satu baca /proc/<pid>/stat -> nama (comm), RSS (field 24, page),
// utime (14) + stime (15).
func procStat(pid int) (name string, rss, utime, stime uint64, ok bool) {
	b, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if err != nil {
		return "", 0, 0, 0, false
	}
	s := string(b)
	lp := strings.IndexByte(s, '(')
	rp := strings.LastIndexByte(s, ')')
	if lp < 0 || rp < lp+1 || rp+2 >= len(s) {
		return "", 0, 0, 0, false
	}
	name = s[lp+1 : rp]
	// setelah ")..." field dimulai dari state (field 3) -> rest[0].
	// utime = field 14 = rest[11], stime = 15 = rest[12], rss = 24 = rest[21].
	rest := strings.Fields(s[rp+2:])
	if len(rest) <= 21 {
		return name, 0, 0, 0, false
	}
	utime, _ = strconv.ParseUint(rest[11], 10, 64)
	stime, _ = strconv.ParseUint(rest[12], 10, 64)
	pages, _ := strconv.ParseUint(rest[21], 10, 64)
	// nama penuh dari cmdline (stat cuma 15 char, bisa merge 2 program beda);
	// fallback comm kalau kosong (kernel thread/zombie).
	if n := cmdName(pid); n != "" {
		name = n
	}
	return name, pages * uint64(os.Getpagesize()), utime, stime, true
}

// cmdName: nama program dari /proc/<pid>/cmdline — argv[0], basename, potong flag.
// comm (stat) terpotong di 15 char, contoh: "xdg-desktop-portal" jadi
// "xdg-desktop-por" → 2 program beda bisa merge. cmdline mengembalikan penuh.
// catatan: argv[0] bisa berisi flag (Chrome: "/path/chrome --type=renderer ...")
// → basename lalu potong di spasi pertama.
func cmdName(pid int) string {
	b, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/cmdline")
	if err != nil || len(b) == 0 || b[0] == 0 {
		return ""
	}
	n := bytes.IndexByte(b, 0)
	if n < 0 {
		n = len(b)
	}
	s := path.Base(string(b[:n]))
	if i := strings.IndexByte(s, ' '); i >= 0 {
		s = s[:i]
	}
	return s
}
