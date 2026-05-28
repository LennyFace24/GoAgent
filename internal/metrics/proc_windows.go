//go:build windows

package metrics

import (
	"github.com/shirou/gopsutil/v4/net"
)

// collectPlatformMetrics 采集 Windows 特有指标
// TCP TIME_WAIT 用 net.Connections 统计 CLOSE_WAIT 作为近似替代
// OOM Kill 在 Windows 上无此概念，返回 nil
func collectPlatformMetrics() (tcpTW *float64, oomKills *float64) {
	tcpTW = collectTcpCloseWait()
	return tcpTW, nil
}

func collectTcpCloseWait() *float64 {
	conns, err := net.Connections("tcp")
	if err != nil {
		return nil
	}
	var count float64
	for _, c := range conns {
		if c.Status == "CLOSE_WAIT" {
			count++
		}
	}
	return &count
}
