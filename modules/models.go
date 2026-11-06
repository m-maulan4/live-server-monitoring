package modules

// CPU metrics (usage % dihitung dari delta /proc/stat antar Collect).
type CPU struct {
	Cores int     `json:"cores"`
	Usage float64 `json:"usage"` // persen 0-100
}

// RAM metrics dari /proc/meminfo (KB).
type RAM struct {
	Total     uint64  `json:"total"`
	Available uint64  `json:"available"`
	Used      uint64  `json:"used"` // Total - Available
	Pct       float64 `json:"pct"`  // used/total*100
}

// Storage metrics agregat (semua mount dijumlah).
type Storage struct {
	Total uint64  `json:"total"`
	Used  uint64  `json:"used"`
	Pct   float64 `json:"pct"` // used/total*100
}

// Zram metrics (byte). Used dari /proc/meminfo (cocok free -h), compressed dari mm_stat.
type Zram struct {
	DiskSize   uint64  `json:"disk_size"`  // kapasitas device (byte)
	Used       uint64  `json:"used"`       // SwapTotal - SwapFree, cocok free -h
	Compressed uint64  `json:"compressed"` // compr_data_size: data setelah kompresi
	Ratio      float64 `json:"ratio"`      // compressed/original*100 (% dari ukuran asli)
	Pct        float64 `json:"pct"`        // used/disk_size*100 (fill%)
}

// Network metrics total semua interface + rate (byte/s -> Mbps dihitung server).
type Network struct {
	RxBytes uint64  `json:"rx_bytes"` // kumulatif rx sejak boot
	TxBytes uint64  `json:"tx_bytes"` // kumulatif tx sejak boot
	RxMbps  float64 `json:"rx_mbps"`  // rate rx
	TxMbps  float64 `json:"tx_mbps"`  // rate tx
	NetMbps float64 `json:"net_mbps"` // rx+tx
}

// Server info statis (hostname, OS, CPU topology) + uptime/load.
type Server struct {
	Hostname string  `json:"hostname"`
	OS       string  `json:"os"`
	CPUModel string  `json:"cpu_model"`
	Cores    int     `json:"cores"`   // core fisik
	Threads  int     `json:"threads"` // thread logika (termasuk hyperthread)
	Uptime   float64 `json:"uptime"`  // detik sejak boot
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
