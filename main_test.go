package main

import (
	"encoding/json"
	"testing"
)

// TestHubLifecycle: subscribe/unsubscribe mengubah empty() sesuai jumlah client.
func TestHubLifecycle(t *testing.T) {
	h := newHub()
	if !h.empty() {
		t.Fatal("hub baru harus empty")
	}
	c1 := h.subscribe()
	c2 := h.subscribe()
	if h.empty() {
		t.Fatal("harusnya ada client setelah subscribe")
	}
	h.unsubscribe(c1)
	if h.empty() {
		t.Fatal("masih ada 1 client, tidak boleh empty")
	}
	h.unsubscribe(c2)
	if !h.empty() {
		t.Fatal("harusnya empty setelah semua unsubscribe")
	}
}

// TestSubscribeLatest: client baru langsung terima snapshot terakhir.
func TestSubscribeLatest(t *testing.T) {
	h := newHub()
	h.broadcast([]byte("hello"))
	ch := h.subscribe()
	got := <-ch
	if string(got) != "hello" {
		t.Fatalf("expected hello, got %q", got)
	}
}

// TestBroadcastNonBlocking: client lambat di-drop, bukan memblok producer.
func TestBroadcastNonBlocking(t *testing.T) {
	h := newHub()
	ch := h.subscribe()
	// isi buffer chan (kapasitas 1) tanpa dibaca
	h.broadcast([]byte("a"))
	h.broadcast([]byte("b")) // ch penuh -> client di-drop, tak deadlock
	if !h.empty() {
		t.Fatal("client lambat harus di-drop")
	}
	_ = ch
}

// TestSnapshotJSON: snapshot ter-serialize ke JSON valid dengan field wajib.
func TestSnapshotJSON(t *testing.T) {
	b := collect()
	if b == nil {
		t.Fatal("collect() nil")
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("snapshot bukan JSON valid: %v", err)
	}
	for _, k := range []string{"server", "cpu", "ram", "storage", "zram", "network", "logs", "processes"} {
		if _, ok := m[k]; !ok {
			t.Errorf("field wajib %q hilang", k)
		}
	}
}
