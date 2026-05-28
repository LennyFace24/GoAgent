package metrics

import (
	"math"
	"strings"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/load"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/net"
)

// GopsutilCollector 通过 gopsutil 采集系统指标
type GopsutilCollector struct {
	mu sync.RWMutex

	// 速率指标缓存（由后台 goroutine 更新）
	cpuPercent *float64
	netRxRate  *float64
	netTxRate  *float64
	diskIORate *float64

	// 上一次采样值，用于计算速率
	prevNetRx     uint64
	prevNetTx     uint64
	prevDiskIO    uint64
	prevSampleTime time.Time
}

// NewCollector 创建采集器并启动后台采样 goroutine
func NewCollector() Collector {
	c := &GopsutilCollector{
		prevSampleTime: time.Now(),
	}
	// 启动后台采样，每 5 秒采集一次速率指标
	go c.samplingLoop()
	return c
}

func (c *GopsutilCollector) samplingLoop() {
	// 首次采样建立基线
	c.sample()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		c.sample()
	}
}

func (c *GopsutilCollector) sample() {
	now := time.Now()
	elapsed := now.Sub(c.prevSampleTime).Seconds()
	if elapsed < 1 {
		elapsed = 1
	}

	// CPU
	cpuPercents, err := cpu.Percent(0, false)
	var cpuVal *float64
	if err == nil && len(cpuPercents) > 0 {
		cpuVal = round1(cpuPercents[0])
	}

	// 网络
	var netRx, netTx *float64
	ioCounters, err := net.IOCounters(false)
	if err == nil && len(ioCounters) > 0 {
		total := ioCounters[0]
		if c.prevNetRx > 0 {
			rx := float64(total.BytesRecv-c.prevNetRx) * 8 / elapsed / 1e6
			netRx = round1(rx)
		}
		if c.prevNetTx > 0 {
			tx := float64(total.BytesSent-c.prevNetTx) * 8 / elapsed / 1e6
			netTx = round1(tx)
		}
		c.prevNetRx = total.BytesRecv
		c.prevNetTx = total.BytesSent
	}

	// 磁盘 IO
	var diskIO *float64
	diskIOCounters, err := disk.IOCounters()
	if err == nil {
		var totalIO uint64
		for _, v := range diskIOCounters {
			totalIO += v.IoTime
		}
		if c.prevDiskIO > 0 {
			util := float64(totalIO-c.prevDiskIO) / elapsed / 10 // IoTime 单位 ms，除以 10 得百分比
			if util > 100 {
				util = 100
			}
			diskIO = round1(util)
		}
		c.prevDiskIO = totalIO
	}

	c.prevSampleTime = now

	c.mu.Lock()
	c.cpuPercent = cpuVal
	c.netRxRate = netRx
	c.netTxRate = netTx
	c.diskIORate = diskIO
	c.mu.Unlock()
}

// Collect 采集所有系统指标
func (c *GopsutilCollector) Collect() (*MetricsSnapshot, error) {
	snap := &MetricsSnapshot{}

	// 从缓存读取速率指标
	c.mu.RLock()
	snap.CPU = c.cpuPercent
	snap.NetworkRx = c.netRxRate
	snap.NetworkTx = c.netTxRate
	snap.DiskIOUtil = c.diskIORate
	c.mu.RUnlock()

	// 内存
	if v, err := mem.VirtualMemory(); err == nil {
		snap.Memory = round1(v.UsedPercent)
		totalGB := float64(v.Total) / 1e9
		usedGB := float64(v.Used) / 1e9
		snap.MemTotalGB = round1(totalGB)
		snap.MemUsedGB = round1(usedGB)
	}

	// Swap
	if v, err := mem.SwapMemory(); err == nil && v.Total > 0 {
		snap.SwapUse = round1(v.UsedPercent)
	}

	// 磁盘使用率 + Inode
	snap.Disk, snap.InodeUse = c.collectDiskUsage()

	// 系统负载
	if v, err := load.Avg(); err == nil {
		snap.Load1m = round1(v.Load1)
		snap.Load5m = round1(v.Load5)
		snap.Load15m = round1(v.Load15)
	}

	// 平台特定指标（TCP TIME_WAIT、OOM Kill）
	snap.TcpTimeWait, snap.OomKills1h = collectPlatformMetrics()

	return snap, nil
}

// collectDiskUsage 获取物理磁盘使用率，过滤 tmpfs/overlay/squashfs/fuse
func (c *GopsutilCollector) collectDiskUsage() (diskPct *float64, inodePct *float64) {
	partitions, err := disk.Partitions(false)
	if err != nil {
		return nil, nil
	}

	excludeFstypes := map[string]bool{
		"tmpfs": true, "squashfs": true, "overlay": true,
	}

	var bestUsage *disk.UsageStat
	var bestSize uint64

	for _, p := range partitions {
		// 过滤非物理文件系统
		if excludeFstypes[p.Fstype] || strings.HasPrefix(p.Fstype, "fuse") {
			continue
		}
		u, err := disk.Usage(p.Mountpoint)
		if err != nil {
			continue
		}
		// 选最大的分区（通常是根分区或主数据分区）
		if u.Total > bestSize {
			bestSize = u.Total
			bestUsage = u
		}
	}

	if bestUsage == nil {
		return nil, nil
	}

	return round1(bestUsage.UsedPercent), round1(bestUsage.InodesUsedPercent)
}

// round1 保留 1 位小数，NaN/Inf 返回 nil
func round1(v float64) *float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return nil
	}
	r := float64(int(v*10)) / 10
	return &r
}
