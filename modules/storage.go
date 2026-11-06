package modules

import "syscall"

// StorageUsage: agregat semua path (default "/"). Total+Used+pct dihitung di sini.
func StorageUsage(paths ...string) (Storage, error) {
	if len(paths) == 0 {
		paths = []string{"/"} // ponytail: root saja; tambah mount list kalau perlu
	}
	var agg Storage
	for _, p := range paths {
		var st syscall.Statfs_t
		if err := syscall.Statfs(p, &st); err != nil {
			continue
		}
		total := st.Blocks * uint64(st.Bsize)
		free := st.Bavail * uint64(st.Bsize)
		agg.Total += total
		agg.Used += total - free
	}
	if agg.Total > 0 {
		agg.Pct = float64(agg.Used) / float64(agg.Total) * 100
	}
	return agg, nil
}
