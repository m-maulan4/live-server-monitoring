package modules

import (
	"bytes"
	"context"
	"encoding/json"
	"os/exec"
	"strconv"
	"sync"
	"time"
)

var (
	logMu       sync.Mutex
	logCursor   string           // cursor journald entry terakhir yang sudah dikirim
	lastLogs    []Log             // cache: kirim saat journalctl gagal transien
)

const maxLogs = 50 // simpan 50 terbaru, yang lama dibuang

// journalEntry cocok format -o json journald.
type journalEntry struct {
	Cursor    string          `json:"__CURSOR"`
	Realtime  json.RawMessage `json:"__REALTIME_TIMESTAMP"`
	Unit      string          `json:"_SYSTEMD_UNIT"`
	Ident     string          `json:"SYSLOG_IDENTIFIER"`
	Priority  string          `json:"PRIORITY"`
	Message   json.RawMessage `json:"MESSAGE"`
}

// SystemLog: panggilan pertama kirim `n` entry terakhir, lalu hanya entry
// BARU (via journalctl --after-cursor). Gagal transien → kirim cache.
func SystemLog(n int) ([]Log, error) {
	logMu.Lock()
	defer logMu.Unlock()

	args := []string{"-o", "json", "--no-pager"}
	if logCursor == "" {
		args = append(args, "-n", strconv.Itoa(n))
	} else {
		args = append(args, "--after-cursor", logCursor)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "journalctl", args...).Output()
	if err != nil {
		if logCursor != "" {
			logCursor = "" // cursor basi (journald rotate) → full fetch berikutnya
		}
		return lastLogs, nil // kirim cache, jangan kosongkan UI
	}

	var logs []Log
	for len(out) > 0 {
		var line []byte
		if i := bytes.IndexByte(out, '\n'); i >= 0 {
			line, out = out[:i], out[i+1:]
		} else {
			line, out = out, nil
		}
		if len(line) == 0 {
			continue
		}
		var e journalEntry
		if json.Unmarshal(line, &e) != nil {
			continue
		}
		if e.Cursor != "" {
			logCursor = e.Cursor
		}
		logs = append(logs, Log{
			Time:     tsString(e.Realtime),
			Unit:     firstNonEmpty(e.Unit, e.Ident),
			Priority: e.Priority,
			Message:  msgString(e.Message),
			Cursor:   e.Cursor,
		})
	}
	if len(logs) > maxLogs {
		logs = logs[len(logs)-maxLogs:]
	}
	lastLogs = logs
	return logs, nil
}

// msgString: MESSAGE bisa string atau array byte (karakter kontrol).
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

// tsString: __REALTIME_TIMESTAMP = microsecond epoch string/number; ubah ke RFC3339.
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

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
