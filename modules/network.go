package modules

import (
	"os"
	"strconv"
	"strings"
)

// NetworkUsage membaca /proc/net/dev, kembalikan rx/tx kumulatif per interface.
// ponytail: kumulatif sejak boot; untuk rate (byte/s) perlu delta sampling 2x seperti CPU.
func NetworkUsage() ([]Network, error) {
	b, err := os.ReadFile("/proc/net/dev")
	if err != nil {
		return nil, err
	}
	var out []Network
	for _, line := range strings.Split(string(b), "\n") {
		i := strings.Index(line, ":")
		if i < 0 {
			continue
		}
		f := strings.Fields(line[i+1:])
		if len(f) < 9 {
			continue
		}
		rx, _ := strconv.ParseUint(f[0], 10, 64)
		tx, _ := strconv.ParseUint(f[8], 10, 64)
		out = append(out, Network{RxBytes: rx, TxBytes: tx})
	}
	return out, nil
}
