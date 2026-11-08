package modules

import (
	"encoding/json"
	"testing"
	"time"
)

// ---- pure helpers ----

func TestMemKb(t *testing.T) {
	cases := map[string]uint64{
		"MemTotal:       16384000 kB": 16384000,
		"SwapFree:       0 kB":        0,
		"":                            0,
		"bad":                         0,
	}
	for line, want := range cases {
		if got := memKb(line); got != want {
			t.Errorf("memKb(%q) = %d, want %d", line, got, want)
		}
	}
}

func TestFirstNonEmpty(t *testing.T) {
	if got := firstNonEmpty("", "b"); got != "b" {
		t.Errorf("firstNonEmpty(\"\",\"b\") = %q", got)
	}
	if got := firstNonEmpty("a", "b"); got != "a" {
		t.Errorf("firstNonEmpty(\"a\",\"b\") = %q", got)
	}
}

func TestMsgString(t *testing.T) {
	// string langsung
	if got := msgString(json.RawMessage(`"hi"`)); got != "hi" {
		t.Errorf("string msg = %q", got)
	}
	// array byte (karakter kontrol)
	if got := msgString(json.RawMessage(`[104,105]`)); got != "hi" {
		t.Errorf("byte-array msg = %q", got)
	}
	// invalid
	if got := msgString(json.RawMessage(`123`)); got != "" {
		t.Errorf("invalid msg = %q", got)
	}
}

func TestTsString(t *testing.T) {
	// hasil pakai timezone lokal host (bukan selalu UTC) — bandingkan parsed epoch
	const us = int64(1700000000000000)
	want, _ := time.Parse(time.RFC3339, time.UnixMicro(us).Format(time.RFC3339))
	for _, raw := range []string{`1700000000000000`, `"1700000000000000"`} {
		got := tsString(json.RawMessage(raw))
		parsed, err := time.Parse(time.RFC3339, got)
		if err != nil {
			t.Fatalf("tsString(%s) = %q, bukan RFC3339", raw, got)
		}
		if !parsed.Equal(want) {
			t.Errorf("tsString(%s) = %q, want epoch %d", raw, got, us)
		}
	}
	// zero -> ""
	if got := tsString(json.RawMessage(`0`)); got != "" {
		t.Errorf("zero ts = %q", got)
	}
}

// ---- smoke: baca /proc nyata, assert wajar (skip kalau tak ada /proc) ----

func TestRAMUsage(t *testing.T) {
	r, err := RAMUsage()
	if err != nil {
		t.Skipf("RAMUsage err (bukan Linux?): %v", err)
	}
	if r.Total == 0 {
		t.Fatal("Total = 0, /proc/meminfo tak terbaca")
	}
	if r.Used > r.Total {
		t.Errorf("Used(%d) > Total(%d)", r.Used, r.Total)
	}
	if r.Pct < 0 || r.Pct > 100 {
		t.Errorf("Pct = %f di luar 0..100", r.Pct)
	}
}

func TestStorageUsage(t *testing.T) {
	s, err := StorageUsage()
	if err != nil {
		t.Skipf("StorageUsage err: %v", err)
	}
	if s.Total == 0 {
		t.Fatal("Total = 0")
	}
	if s.Used > s.Total {
		t.Errorf("Used(%d) > Total(%d)", s.Used, s.Total)
	}
}

func TestNetworkUsage(t *testing.T) {
	n, err := NetworkUsage()
	if err != nil {
		t.Skipf("NetworkUsage err: %v", err)
	}
	// rx/tx kumulatif bisa 0 di container terisolasi; hanya cek tak negatif rate
	if n.RxMbps < 0 || n.TxMbps < 0 {
		t.Errorf("rate negatif: rx=%f tx=%f", n.RxMbps, n.TxMbps)
	}
}

func TestProcesses(t *testing.T) {
	ps, err := Processes(10)
	if err != nil {
		t.Skipf("Processes err: %v", err)
	}
	if len(ps) > 10 {
		t.Errorf("len = %d, top=10 harusnya <=10", len(ps))
	}
	// RAM descending sudah dijamin fungsi; pastikan tidak negatif
	for _, p := range ps {
		if p.PID <= 0 {
			t.Errorf("PID invalid: %d", p.PID)
		}
	}
}

func TestCPUUsageCache(t *testing.T) {
	// InitCPU tak dijalankan di sini; CPUUsage cukup kembalikan cache (tak panic)
	c, err := CPUUsage()
	if err != nil {
		t.Fatalf("CPUUsage err: %v", err)
	}
	if c.Cores < 0 {
		t.Errorf("Cores negatif: %d", c.Cores)
	}
}
