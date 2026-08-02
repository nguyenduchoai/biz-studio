package util

import (
	"context"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

// HostStats — thống kê máy cho status bar.
type HostStats struct {
	CPUPct    float64 `json:"cpuPct"`
	RAMPct    float64 `json:"ramPct"`
	RAMUsedMB int     `json:"ramUsedMB"`
	DiskPct   float64 `json:"diskPct"`
}

var (
	statsMu   sync.Mutex
	statsAt   time.Time
	statsLast HostStats
)

// Stats trả HostStats, cache 3 giây (dùng ps/vm_stat/df — macOS).
func Stats() HostStats {
	statsMu.Lock()
	defer statsMu.Unlock()
	if time.Since(statsAt) < 3*time.Second {
		return statsLast
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	statsLast = HostStats{CPUPct: cpuPct(ctx), DiskPct: diskPct(ctx)}
	statsLast.RAMPct, statsLast.RAMUsedMB = ramInfo(ctx)
	statsAt = time.Now()
	return statsLast
}

func cpuPct(ctx context.Context) float64 {
	out, err := Run(ctx, "sh", "-c", "ps -A -o %cpu | awk '{s+=$1} END {print s}'")
	if err != nil {
		return 0
	}
	v, _ := strconv.ParseFloat(strings.TrimSpace(out), 64)
	n := float64(runtime.NumCPU())
	if n == 0 {
		n = 1
	}
	pct := v / n
	if pct > 100 {
		pct = 100
	}
	return round1(pct)
}

func ramInfo(ctx context.Context) (float64, int) {
	total, err := Run(ctx, "sysctl", "-n", "hw.memsize")
	if err != nil {
		return 0, 0
	}
	tb, _ := strconv.ParseInt(strings.TrimSpace(total), 10, 64)
	out, err := Run(ctx, "sh", "-c",
		`vm_stat | awk '/Pages (active|wired down|occupied by compressor)/ {gsub("\\.",""); s+=$NF} END {print s*16384}'`)
	if err != nil || tb == 0 {
		return 0, 0
	}
	used, _ := strconv.ParseInt(strings.TrimSpace(out), 10, 64)
	return round1(float64(used) / float64(tb) * 100), int(used / 1024 / 1024)
}

func diskPct(ctx context.Context) float64 {
	out, err := Run(ctx, "sh", "-c", "df -k / | tail -1 | awk '{print $5}' | tr -d '%'")
	if err != nil {
		return 0
	}
	v, _ := strconv.ParseFloat(strings.TrimSpace(out), 64)
	return v
}

func round1(v float64) float64 { return float64(int(v*10)) / 10 }
