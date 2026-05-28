//go:build linux

package metrics

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	oomMu         sync.Mutex
	oomSamples    []oomSample
	oomSampleTTL  = 10 * time.Minute
)

type oomSample struct {
	count uint64
	time  time.Time
}

// collectPlatformMetrics 采集 Linux 特有指标：TCP TIME_WAIT + OOM Kill
func collectPlatformMetrics() (tcpTW *float64, oomKills *float64) {
	tcpTW = collectTcpTimeWait()
	oomKills = collectOomKills()
	return
}

// collectTcpTimeWait 解析 /proc/net/sockstat 获取 TCP TIME_WAIT 数
func collectTcpTimeWait() *float64 {
	f, err := os.Open("/proc/net/sockstat")
	if err != nil {
		return nil
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "TCP:") {
			// 格式: TCP: inuse X orphan X tw X alloc X mem X
			fields := strings.Fields(line)
			for i := 0; i < len(fields)-1; i++ {
				if fields[i] == "tw" {
					v, err := strconv.ParseUint(fields[i+1], 10, 64)
					if err != nil {
						return nil
					}
					fv := float64(v)
					return &fv
				}
			}
		}
	}
	return nil
}

// collectOomKills 解析 /proc/vmstat 获取 oom_kill 计数，计算 1 小时增量
func collectOomKills() *float64 {
	count, err := readOomKillCount()
	if err != nil {
		return nil
	}

	now := time.Now()

	oomMu.Lock()
	defer oomMu.Unlock()

	// 清理过期样本
	cutoff := now.Add(-oomSampleTTL)
	valid := oomSamples[:0]
	for _, s := range oomSamples {
		if s.time.After(cutoff) {
			valid = append(valid, s)
		}
	}
	oomSamples = valid

	// 添加当前样本
	oomSamples = append(oomSamples, oomSample{count: count, time: now})

	// 计算 1 小时增量：用最新值减去最旧值（如果样本跨度够长）
	if len(oomSamples) < 2 {
		zero := 0.0
		return &zero
	}

	oldest := oomSamples[0]
	latest := oomSamples[len(oomSamples)-1]

	if latest.count < oldest.count {
		// 计数器重置
		zero := 0.0
		return &zero
	}

	delta := float64(latest.count - oldest.count)

	// 如果采样时间跨度不足 1 小时，按比例外推到 1 小时
	span := latest.time.Sub(oldest.time).Seconds()
	if span > 0 && span < 3600 {
		delta = delta * 3600 / span
	}

	return round1(delta)
}

// readOomKillCount 从 /proc/vmstat 读取 oom_kill 值
func readOomKillCount() (uint64, error) {
	f, err := os.Open("/proc/vmstat")
	if err != nil {
		return 0, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "oom_kill ") {
			parts := strings.Fields(line)
			if len(parts) != 2 {
				return 0, fmt.Errorf("unexpected oom_kill line: %s", line)
			}
			return strconv.ParseUint(parts[1], 10, 64)
		}
	}
	return 0, fmt.Errorf("oom_kill not found in /proc/vmstat")
}
