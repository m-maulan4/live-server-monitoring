package modules

import (
	"math"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	netMu     sync.Mutex
	netPrevRx uint64
	netPrevTx uint64
	netPrevT  time.Time
)

// NetworkUsage: total rx/tx kumulatif semua interface + rate (Mbps) via delta
// antar panggilan. Rate dihitung server, bukan di FE.
// ponytail: pertama kali dipanggil rate = 0 (belum ada sampel sebelumnya).
func NetworkUsage() (Network, error) {
	b, err := os.ReadFile("/proc/net/dev")
	if err != nil {
		return Network{}, err
	}
	var rx, tx uint64
	for _, line := range strings.Split(string(b), "\n") {
		i := strings.Index(line, ":")
		if i < 0 {
			continue
		}
		f := strings.Fields(line[i+1:])
		if len(f) < 9 {
			continue
		}
		r, _ := strconv.ParseUint(f[0], 10, 64)
		t, _ := strconv.ParseUint(f[8], 10, 64)
		rx += r
		tx += t
	}

	n := Network{RxBytes: rx, TxBytes: tx}

	netMu.Lock()
	now := time.Now()
	if !netPrevT.IsZero() {
		dt := now.Sub(netPrevT).Seconds()
		if dt > 0 {
			n.RxMbps = math.Max(0, float64(rx-netPrevRx)*8/dt/1e6)
			n.TxMbps = math.Max(0, float64(tx-netPrevTx)*8/dt/1e6)
			n.NetMbps = n.RxMbps + n.TxMbps
		}
	}
	netPrevRx, netPrevTx, netPrevT = rx, tx, now
	netMu.Unlock()

	return n, nil
}
