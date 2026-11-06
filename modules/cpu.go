package modules

import (
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

type cpuTimes struct {
	cores int
	idle  uint64
	total uint64
}

var (
	cpuMu   sync.RWMutex
	cpuStat = CPU{}
)

// InitCPU mulai kolektor background: sampling /proc/stat tiap 500ms,
// hitung usage dari delta antar sample. Panggil sekali di main().
// CPUUsage() lalu instan (baca cache), tanpa sleep di jalur request.
func InitCPU() {
	go func() {
		var prev cpuTimes
		for {
			cur, err := readCpuTimes()
			if err == nil {
				var c CPU
				c.Cores = cur.cores
				if prev.total > 0 && cur.total > prev.total {
					idle := cur.idle - prev.idle
					total := cur.total - prev.total
					if total > 0 {
						c.Usage = (1 - float64(idle)/float64(total)) * 100
					}
				}
				cpuMu.Lock()
				cpuStat = c
				cpuMu.Unlock()
				prev = cur
			}
			time.Sleep(500 * time.Millisecond)
		}
	}()
}

// CPUUsage kembalikan cache CPU (instan).
func CPUUsage() (CPU, error) {
	cpuMu.RLock()
	defer cpuMu.RUnlock()
	return cpuStat, nil
}

func readCpuTimes() (cpuTimes, error) {
	b, err := os.ReadFile("/proc/stat")
	if err != nil {
		return cpuTimes{}, err
	}
	var r cpuTimes
	for _, line := range strings.Split(string(b), "\n") {
		f := strings.Fields(line)
		if len(f) < 5 {
			continue
		}
		if f[0] == "cpu" {
			for _, v := range f[1:] {
				n, _ := strconv.ParseUint(v, 10, 64)
				r.total += n
			}
			idle, _ := strconv.ParseUint(f[4], 10, 64)
			iowait, _ := strconv.ParseUint(f[5], 10, 64)
			r.idle = idle + iowait
		}
		if strings.HasPrefix(f[0], "cpu") && f[0] != "cpu" {
			r.cores++
		}
	}
	return r, nil
}
