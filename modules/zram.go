package modules

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// ZramUsage menjumlahkan semua device zram*.
func ZramUsage() (Zram, error) {
	z := Zram{}
	paths, err := filepath.Glob("/sys/block/zram*/mm_stat")
	if err != nil || len(paths) == 0 {
		return z, err // zram tidak ada/ter-disable
	}
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		f := strings.Fields(string(b))
		if len(f) < 4 {
			continue
		}
		d, _ := strconv.ParseUint(f[0], 10, 64)
		o, _ := strconv.ParseUint(f[1], 10, 64)
		c, _ := strconv.ParseUint(f[2], 10, 64)
		m, _ := strconv.ParseUint(f[3], 10, 64)
		z.DiskSize += d
		z.Original += o
		z.Compressed += c
		z.MemUsedFree += m
	}
	return z, nil
}
