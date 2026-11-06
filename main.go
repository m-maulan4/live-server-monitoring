package main

import (
	"encoding/json"
	"net/http"
	"os"
	"time"

	"statusserver/modules"
)

type snapshot struct {
	Server    modules.Server    `json:"server"`
	CPU       modules.CPU       `json:"cpu"`
	RAM       modules.RAM       `json:"ram"`
	Storage   []modules.Storage `json:"storage"`
	Zram      modules.Zram      `json:"zram"`
	Network   []modules.Network `json:"network"`
	Logs      []modules.Log     `json:"logs"`
	Processes []modules.Process `json:"processes"`
}

func sse(w http.ResponseWriter, r *http.Request) {
	fl, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "no flusher", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	t := time.NewTicker(1 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-t.C:
			s := snapshot{}
			s.Server, _ = modules.ServerInfo()
			s.CPU, _ = modules.CPUUsage()
			s.RAM, _ = modules.RAMUsage()
			s.Storage, _ = modules.StorageUsage()
			s.Zram, _ = modules.ZramUsage()
			s.Network, _ = modules.NetworkUsage()
			s.Logs, _ = modules.SystemLog(30)
			s.Processes, _ = modules.Processes(10)
			b, _ := json.Marshal(s)
			w.Write([]byte("data: " + string(b) + "\n\n"))
			fl.Flush()
		}
	}
}

func main() {
	http.HandleFunc("/health", sse)
	http.Handle("/", http.FileServer(http.Dir("public")))
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	http.ListenAndServe(":"+port, nil)
}
