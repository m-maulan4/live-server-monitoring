package modules

import (
	"os"
	"strconv"
	"strings"
	"sync"
	"syscall"
)

// Bagian statis server (tak berubah seumur hidup proses) dibaca sekali.
var (
	staticOnce   sync.Once
	hostname     string
	osName       string
	cpuModelName string
	coresNum     int
	threadsNum   int
)

func initStatic() {
	if h, err := os.Hostname(); err == nil {
		hostname = h
	}
	if v := osRelease("PRETTY_NAME"); v != "" {
		osName = v
	} else {
		var un syscall.Utsname
		if syscall.Uname(&un) == nil {
			osName = charsToString(un.Sysname[:])
		}
	}
	cpuModelName, coresNum, threadsNum = parseCPUInfo(readFile("/proc/cpuinfo"))
}

// ServerInfo: statis di-cache (sync.Once), dinamis (uptime/load) per panggilan.
func ServerInfo() (Server, error) {
	staticOnce.Do(initStatic)
	s := Server{
		Hostname: hostname,
		OS:       osName,
		CPUModel: cpuModelName,
		Cores:    coresNum,
		Threads:  threadsNum,
	}
	s.Uptime = firstNum("/proc/uptime")
	if f := strings.Fields(readFile("/proc/loadavg")); len(f) >= 3 {
		s.Load1, _ = strconv.ParseFloat(f[0], 64)
		s.Load5, _ = strconv.ParseFloat(f[1], 64)
		s.Load15, _ = strconv.ParseFloat(f[2], 64)
	}
	return s, nil
}

func readFile(p string) string {
	b, err := os.ReadFile(p)
	if err != nil {
		return ""
	}
	return string(b)
}

func firstNum(p string) float64 {
	f := strings.Fields(readFile(p))
	if len(f) == 0 {
		return 0
	}
	n, _ := strconv.ParseFloat(f[0], 64)
	return n
}

func osRelease(key string) string {
	for _, line := range strings.Split(readFile("/etc/os-release"), "\n") {
		if strings.HasPrefix(line, key+"=") {
			return strings.Trim(strings.TrimPrefix(line, key+"="), `"`)
		}
	}
	return ""
}

// parseCPUInfo: satu pass /proc/cpuinfo -> model + core fisik + thread logika.
// ponytail: VM/ARM tanpa physical id/cpu cores → cores = threads (tak bisa dibedakan).
func parseCPUInfo(raw string) (model string, cores, threads int) {
	sockets := map[string]bool{}
	coresPerSocket := 0
	for _, line := range strings.Split(raw, "\n") {
		f := strings.SplitN(line, ":", 2)
		if len(f) != 2 {
			continue
		}
		k, v := strings.TrimSpace(f[0]), strings.TrimSpace(f[1])
		switch k {
		case "model name", "Hardware", "Model":
			if model == "" {
				model = v
			}
		case "processor":
			threads++
		case "physical id":
			sockets[v] = true
		case "cpu cores":
			if n, _ := strconv.Atoi(v); n > coresPerSocket {
				coresPerSocket = n
			}
		}
	}
	if n := len(sockets); n > 0 && coresPerSocket > 0 {
		cores = n * coresPerSocket
	} else if coresPerSocket > 0 {
		cores = coresPerSocket
	} else {
		cores = threads
	}
	if threads == 0 {
		threads = cores
	}
	if threads < cores {
		cores = threads
	}
	return model, cores, threads
}

func charsToString(a []int8) string {
	b := make([]byte, len(a))
	for i, v := range a {
		b[i] = byte(v)
	}
	return strings.TrimRight(string(b), "\x00")
}
