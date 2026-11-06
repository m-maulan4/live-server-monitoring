package modules

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// ZramUsage menjumlahkan semua device zram*. Ratio/pct dihitung di sini.
func ZramUsage() (Zram, error) {
	z := Zram{}
	paths, err := filepath.Glob("/sys/block/zram*")
	if err != nil || len(paths) == 0 {
		return z, err // zram tidak ada/ter-disable
	}
	var orig uint64
	for _, p := range paths {
		// kapasitas device (byte), bukan dari mm_stat
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
	if orig > 0 {
		z.Pct = float64(z.Compressed) / float64(orig) * 100
		if z.Compressed > 0 {
			z.Ratio = float64(orig) / float64(z.Compressed)
		}
	}
	return z, nil
}
