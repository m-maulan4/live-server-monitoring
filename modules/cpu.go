package modules

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type cpuTimes struct {
	cores int
	idle  uint64
	total uint64
}

// CPUUsage membaca /proc/stat dua kali (interval 1s) untuk hitung usage.
// ponytail: poll per-call; kalau butuh presisi, pindah ke collector background.
func CPUUsage() (CPU, error) {
	c := CPU{}
	stat1, err := readCpuTimes()
	if err != nil {
		return c, err
	}
	time.Sleep(1 * time.Second)
	stat2, err := readCpuTimes()
	if err != nil {
		return c, err
	}
	c.Cores = stat2.cores
	idle := stat2.idle - stat1.idle
	total := stat2.total - stat1.total
	if total > 0 {
		c.Usage = (1 - float64(idle)/float64(total)) * 100
	}
	return c, nil
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
