package modules

import (
	"os"
	"strconv"
	"strings"
	"syscall"
)

func ServerInfo() (Server, error) {
	s := Server{}
	if h, err := os.Hostname(); err == nil {
		s.Hostname = h
	}
	var un syscall.Utsname
	if err := syscall.Uname(&un); err == nil {
		if s.OS == "" {
			s.OS = charsToString(un.Sysname[:])
		}
	}
	if v := osRelease("PRETTY_NAME"); v != "" {
		s.OS = v
	}
	s.CPUModel = cpuModel()
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

func cpuModel() string {
	for _, line := range strings.Split(readFile("/proc/cpuinfo"), "\n") {
		f := strings.SplitN(line, ":", 2)
		if len(f) != 2 {
			continue
		}
		switch strings.TrimSpace(f[0]) {
		case "model name", "Hardware", "Model":
			return strings.TrimSpace(f[1])
		}
	}
	return ""
}

func charsToString(a []int8) string {
	b := make([]byte, len(a))
	for i, v := range a {
		b[i] = byte(v)
	}
	return strings.TrimRight(string(b), "\x00")
}
