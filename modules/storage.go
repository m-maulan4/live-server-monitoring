package modules

import "syscall"

func StorageUsage(paths ...string) ([]Storage, error) {
	if len(paths) == 0 {
		paths = []string{"/"} // ponytail: root saja; tambah mount list kalau perlu
	}
	var out []Storage
	for _, p := range paths {
		var st syscall.Statfs_t
		if err := syscall.Statfs(p, &st); err != nil {
			continue
		}
		total := st.Blocks * uint64(st.Bsize)
		free := st.Bavail * uint64(st.Bsize)
		out = append(out, Storage{Path: p, Total: total, Free: free, Used: total - free})
	}
	return out, nil
}
