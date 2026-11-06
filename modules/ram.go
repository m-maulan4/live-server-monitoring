package modules

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// RAMUsage baca /proc/meminfo (semua nilai byte).
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

// ZramUsage menjumlahkan semua device zram*. Used dari /proc/meminfo
// (SwapTotal - SwapFree) supaya cocok free -h; compressed dari mm_stat.
func ZramUsage() (Zram, error) {
	z := Zram{}
	paths, err := filepath.Glob("/sys/block/zram*")
	if err != nil || len(paths) == 0 {
		return z, err // zram tidak ada/ter-disable
	}
	var orig uint64
	for _, p := range paths {
		// kapasitas device (byte)
		if b, err := os.ReadFile(p + "/disksize"); err == nil {
			n, _ := strconv.ParseUint(strings.TrimSpace(string(b)), 10, 64)
			z.DiskSize += n
		}
		// mm_stat: 0 = orig_data_size, 1 = compr_data_size (byte)
		b, err := os.ReadFile(p + "/mm_stat")
		if err != nil {
			continue
		}
		f := strings.Fields(string(b))
		if len(f) < 2 {
			continue
		}
		o, _ := strconv.ParseUint(f[0], 10, 64)
		c, _ := strconv.ParseUint(f[1], 10, 64)
		orig += o
		z.Compressed += c
	}
	z.Used = swapUsed()
	if orig > 0 && z.Compressed > 0 {
		z.Ratio = float64(z.Compressed) / float64(orig) * 100
	}
	if z.DiskSize > 0 {
		z.Pct = float64(z.Used) / float64(z.DiskSize) * 100
	}
	return z, nil
}

// swapUsed = SwapTotal - SwapFree (byte), sama persis dengan "used" dari free -h.
func swapUsed() uint64 {
	b, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0
	}
	var total, free uint64
	for _, line := range strings.Split(string(b), "\n") {
		switch {
		case strings.HasPrefix(line, "SwapTotal:"):
			total = memKb(line) * 1024
		case strings.HasPrefix(line, "SwapFree:"):
			free = memKb(line) * 1024
		}
	}
	if total > free {
		return total - free
	}
	return 0
}

// memKb parse nilai kB dari baris /proc/meminfo.
func memKb(line string) uint64 {
	f := strings.Fields(line)
	if len(f) < 2 {
		return 0
	}
	n, _ := strconv.ParseUint(f[1], 10, 64)
	return n
}