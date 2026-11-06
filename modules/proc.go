package modules

import (
	"os"
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
	RAM  uint64  `json:"ram"` // byte RSS
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

// Processes membaca /proc dan mengembalikan `top` proses teratas by RAM (RSS).
// CPU% per proses dari delta utime+stime antar panggilan; panggilan pertama selalu 0.
// Satu baca /proc/<pid>/stat per PID sudah mencakup nama+RSS+utime+stime.
func Processes(top int) ([]Process, error) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, err
	}
	now := time.Now()

	type row struct {
		p   Process
		rss uint64
	}
	var rows []row
	seen := make(map[string]int) // name -> index di rows (dedupe by nama program)

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
		r := row{Process{PID: pid, Name: name, CPU: cpu, RAM: rss}, rss}
		if idx, ok := seen[name]; ok {
			// program sama muncul beberapa kali (banyak instans): simpan yang RSS terbesar
			if r.rss > rows[idx].rss {
				rows[idx] = r
			}
		} else {
			seen[name] = len(rows)
			rows = append(rows, r)
		}
	}
	procPrev = newPrev

	sort.Slice(rows, func(i, j int) bool { return rows[i].rss > rows[j].rss })
	if top > 0 && len(rows) > top {
		rows = rows[:top]
	}
	out := make([]Process, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.p)
	}
	return out, nil
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
	return name, pages * uint64(os.Getpagesize()), utime, stime, true
}
