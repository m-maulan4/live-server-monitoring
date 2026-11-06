package modules

import (
	"os"
	"strconv"
	"strings"
)

func RAMUsage() (RAM, error) {
	b, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return RAM{}, err
	}
	get := func(key string) uint64 {
		for _, line := range strings.Split(string(b), "\n") {
			if strings.HasPrefix(line, key+":") {
				n, _ := strconv.ParseUint(strings.Fields(line)[1], 10, 64)
				return n * 1024
			}
		}
		return 0
	}
	total, avail := get("MemTotal"), get("MemAvailable")
	used := total - avail
	pct := 0.0
	if total > 0 {
		pct = float64(used) / float64(total) * 100
	}
	return RAM{Total: total, Available: avail, Used: used, Pct: pct}, nil
}
