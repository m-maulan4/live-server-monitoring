package modules

import (
	"encoding/json"
	"os/exec"
	"strconv"
	"sync"
	"time"
)

var (
	logMu     sync.Mutex
	logCursor string // cursor journald entry terakhir yang sudah dikirim
)

// SystemLog: panggilan pertama kirim `n` entry terakhir, lalu hanya entry
// BARU (via journalctl --after-cursor). Data lama tidak dikirim ulang.
func SystemLog(n int) ([]Log, error) {
	logMu.Lock()
	defer logMu.Unlock()

	args := []string{"-o", "json", "--no-pager"}
	if logCursor == "" {
		args = append(args, "-n", strconv.Itoa(n))
	} else {
		args = append(args, "--after-cursor", logCursor)
	}
	out, err := exec.Command("journalctl", args...).Output()
	if err != nil {
		if logCursor != "" {
			logCursor = "" // cursor basi (journald rotate) → full fetch berikutnya
		}
		return nil, err
	}

	var logs []Log
	for _, line := range splitLines(string(out)) {
		var m map[string]json.RawMessage
		if json.Unmarshal([]byte(line), &m) != nil {
			continue
		}
		if c := strField(m["__CURSOR"]); c != "" {
			logCursor = c
		}
		logs = append(logs, Log{
			Time:     tsString(m["__REALTIME_TIMESTAMP"]),
			Unit:     firstNonEmpty(strField(m["_SYSTEMD_UNIT"]), strField(m["SYSLOG_IDENTIFIER"])),
			Priority: strField(m["PRIORITY"]),
			Message:  msgString(m["MESSAGE"]),
		})
	}
	return logs, nil
}

// MESSAGE bisa string atau array byte (kalau ada karakter kontrol).
func msgString(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var b []byte
	if json.Unmarshal(raw, &b) == nil {
		return string(b)
	}
	return ""
}

func strField(raw json.RawMessage) string {
	var s string
	json.Unmarshal(raw, &s)
	return s
}

// __REALTIME_TIMESTAMP = microsecond epoch string/number; ubah ke RFC3339.
func tsString(raw json.RawMessage) string {
	var us int64
	if json.Unmarshal(raw, &us) != nil {
		var s string
		if json.Unmarshal(raw, &s) != nil {
			return ""
		}
		us, _ = strconv.ParseInt(s, 10, 64)
	}
	if us == 0 {
		return ""
	}
	return time.UnixMicro(us).Format(time.RFC3339)
}

func splitLines(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			if i > start {
				out = append(out, s[start:i])
			}
			start = i + 1
		}
	}
	if start < len(s) {
		out = append(out, s[start:])
	}
	return out
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
