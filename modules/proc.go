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
// CPU% per proses dihitung dari delta utime+stime antar panggilan; panggilan pertama selalu 0.
// ponytail: baca /proc semua PID tiap tick (O(n) file open). Cukup untuk dashboard;
// kalau PID ribuan dan tick < 1s, pindah ke collector background + cache.
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
		name := procName(pid)
		rss := procRSS(pid)
		utime, stime, ok := procTimes(pid)
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

func procName(pid int) string {
	b, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/comm")
	if err != nil {
		return "?"
	}
	return strings.TrimSpace(string(b))
}

func procRSS(pid int) uint64 {
	b, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/status")
	if err != nil {
		return 0
	}
	for _, line := range strings.Split(string(b), "\n") {
		f := strings.Fields(line)
		if len(f) >= 2 && f[0] == "VmRSS:" {
			kb, _ := strconv.ParseUint(f[1], 10, 64)
			return kb * 1024
		}
	}
	return 0
}

func procTimes(pid int) (utime, stime uint64, ok bool) {
	b, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if err != nil {
		return 0, 0, false
	}
	s := string(b)
	i := strings.LastIndexByte(s, ')')
	if i < 0 || i+2 >= len(s) {
		return 0, 0, false
	}
	// setelah ")pid(...)" -> fields mulai dari state (field 3).
	// field 14 = utime (rest[11]), field 15 = stime (rest[12]).
	rest := strings.Fields(s[i+2:])
	if len(rest) <= 12 {
		return 0, 0, false
	}
	utime, _ = strconv.ParseUint(rest[11], 10, 64)
	stime, _ = strconv.ParseUint(rest[12], 10, 64)
	return utime, stime, true
}
