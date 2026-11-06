package main

import (
	"encoding/json"
	"net/http"
	"os"
	"sync"
	"time"

	"statusserver/modules"
)

type snapshot struct {
	Server    modules.Server    `json:"server"`
	CPU       modules.CPU       `json:"cpu"`
	RAM       modules.RAM       `json:"ram"`
	Storage   modules.Storage   `json:"storage"`
	Zram      modules.Zram      `json:"zram"`
	Network   modules.Network   `json:"network"`
	Logs      []modules.Log     `json:"logs"`
	Processes []modules.Process `json:"processes"`
}

var (
	ssePrefix = []byte("data: ")
	sseSuffix = []byte("\n\n")
)

type hub struct {
	mu      sync.Mutex
	clients map[chan []byte]struct{}
	latest  []byte
}

func newHub() *hub { return &hub{clients: make(map[chan []byte]struct{})} }

// subscribe: daftar client, kirim snapshot terakhir langsung (tak nunggu 1s).
func (h *hub) subscribe() chan []byte {
	ch := make(chan []byte, 1)
	h.mu.Lock()
	h.clients[ch] = struct{}{}
	latest := h.latest
	h.mu.Unlock()
	if latest != nil {
		ch <- latest
	}
	return ch
}

func (h *hub) unsubscribe(ch chan []byte) {
	h.mu.Lock()
	delete(h.clients, ch)
	h.mu.Unlock()
}

// broadcast: non-blocking — client lambat di-drop, producer tak boleh diblok.
func (h *hub) broadcast(b []byte) {
	h.mu.Lock()
	h.latest = b
	for ch := range h.clients {
		select {
		case ch <- b:
		default:
			delete(h.clients, ch)
		}
	}
	h.mu.Unlock()
}

func (h *hub) empty() bool {
	h.mu.Lock()
	n := len(h.clients)
	h.mu.Unlock()
	return n == 0
}

func collect() []byte {
	s := snapshot{}
	s.Server, _ = modules.ServerInfo()
	s.CPU, _ = modules.CPUUsage()
	s.RAM, _ = modules.RAMUsage()
	s.Storage, _ = modules.StorageUsage()
	s.Zram, _ = modules.ZramUsage()
	s.Network, _ = modules.NetworkUsage()
	s.Logs, _ = modules.SystemLog(50)
	s.Processes, _ = modules.Processes(10)
	b, _ := json.Marshal(s)
	return b
}

func startProducer(h *hub) {
	t := time.NewTicker(1 * time.Second)
	defer t.Stop()
	for range t.C {
		if h.empty() {
			continue // tak ada penonton → jangan fork journalctl / scan /proc
		}
		h.broadcast(collect())
	}
}

func sse(h *hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fl, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "no flusher", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache, no-transform")
		w.Header().Set("Connection", "keep-alive")
		ch := h.subscribe()
		defer h.unsubscribe(ch)
		fl.Flush() // dorong header segera; Cloudflare butuh flush-per-event

		for {
			select {
			case <-r.Context().Done():
				return
			case b := <-ch:
				w.Write(ssePrefix)
				w.Write(b)
				w.Write(sseSuffix)
				fl.Flush()
			}
		}
	}
}

func main() {
	modules.InitCPU()
	h := newHub()
	go startProducer(h)
	http.HandleFunc("/health", sse(h))
	http.Handle("/", http.FileServer(http.Dir("public")))
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	http.ListenAndServe(":"+port, nil)
}