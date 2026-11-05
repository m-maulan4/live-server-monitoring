package modules

// CPU metrics (usage % dihitung dari delta /proc/stat antar Collect).
type CPU struct {
	Cores int     `json:"cores"`
	Usage float64 `json:"usage"` // persen 0-100
}

// RAM metrics dari /proc/meminfo (KB).
type RAM struct {
	Total     uint64 `json:"total"`
	Available uint64 `json:"available"`
	Used      uint64 `json:"used"` // Total - Available
}

// Storage metrics dari statfs.
type Storage struct {
	Path  string `json:"path"`
	Total uint64 `json:"total"`
	Used  uint64 `json:"used"`
	Free  uint64 `json:"free"`
}

// Zram metrics dari /sys/block/zram0 (byte).
type Zram struct {
	DiskSize    uint64 `json:"disk_size"`
	Original    uint64 `json:"original"`   // data sebelum kompresi
	Compressed  uint64 `json:"compressed"` // data setelah kompresi
	MemUsedFree uint64 `json:"mem_used_free"`
}

// Network metrics per interface dari /proc/net/dev (byte kumulatif sejak boot).
type Network struct {
	Name    string `json:"name"`
	RxBytes uint64 `json:"rx_bytes"`
	TxBytes uint64 `json:"tx_bytes"`
}

// Server info statis (hostname, OS, kernel) + uptime/load.
type Server struct {
	Hostname string  `json:"hostname"`
	OS       string  `json:"os"`
	Kernel   string  `json:"kernel"`
	CPUModel string  `json:"cpu_model"`
	Uptime   float64 `json:"uptime"` // detik sejak boot
	Load1    float64 `json:"load1"`
	Load5    float64 `json:"load5"`
	Load15   float64 `json:"load15"`
}

// Log entry dari journalctl.
type Log struct {
	Time     string `json:"time"`
	Unit     string `json:"unit"`
	Priority string `json:"priority"`
	Message  string `json:"message"`
}
