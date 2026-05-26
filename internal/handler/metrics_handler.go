package handler

import (
	"math"
	"net/http"

	"github.com/LennyFace24/MiniAgent/internal/util"
	"github.com/gin-gonic/gin"
)

type MetricsResponse struct {
	Status       string             `json:"status"`
	CPU          *float64           `json:"cpu,omitempty"`
	Memory       *float64           `json:"memory,omitempty"`
	MemTotalGB   *float64           `json:"mem_total_gb,omitempty"`
	MemUsedGB    *float64           `json:"mem_used_gb,omitempty"`
	Disk         *float64           `json:"disk,omitempty"`
	DiskIOUtil   *float64           `json:"disk_io_util,omitempty"`
	InodeUse     *float64           `json:"inode_use,omitempty"`
	SwapUse      *float64           `json:"swap_use,omitempty"`
	Load1m       *float64           `json:"load1m,omitempty"`
	Load5m       *float64           `json:"load5m,omitempty"`
	Load15m      *float64           `json:"load15m,omitempty"`
	NetworkRx    *float64           `json:"network_rx,omitempty"`
	NetworkTx    *float64           `json:"network_tx,omitempty"`
	TcpTimeWait  *float64           `json:"tcp_tw,omitempty"`
	OomKills1h   *float64           `json:"oom_kills_1h,omitempty"`
	UpCount      int                `json:"up_count"`
	DownCount    int                `json:"down_count"`
	Targets      []util.TargetState `json:"targets,omitempty"`
	Error        string             `json:"error,omitempty"`
}

type MetricsHandler struct {
	pc *util.PromClient
}

func NewMetricsHandler(promURL string) *MetricsHandler {
	return &MetricsHandler{pc: util.NewPromClient(promURL)}
}

func (h *MetricsHandler) GetMetrics(c *gin.Context) {
	if h.pc.BaseURL == "" {
		c.JSON(http.StatusOK, MetricsResponse{
			Status: "error",
			Error:  "Prometheus 未配置",
		})
		return
	}

	resp := MetricsResponse{Status: "ok"}

	resp.CPU = round1(h.pc.InstantQuery(`100 - (avg(rate(node_cpu_seconds_total{mode="idle"}[5m])) * 100)`))
	resp.Memory = round1(h.pc.InstantQuery(`(1 - node_memory_MemAvailable_bytes / node_memory_MemTotal_bytes) * 100`))
	resp.MemTotalGB = round1(h.pc.InstantQuery(`node_memory_MemTotal_bytes / 1e9`))
	resp.MemUsedGB = round1(h.pc.InstantQuery(`(node_memory_MemTotal_bytes - node_memory_MemAvailable_bytes) / 1e9`))
	resp.Disk = round1(h.pc.InstantQuery(`(1 - node_filesystem_avail_bytes{fstype!~"tmpfs|squashfs|overlay|fuse.*"} / node_filesystem_size_bytes{fstype!~"tmpfs|squashfs|overlay|fuse.*"}) * 100`))
	resp.Load1m = round1(h.pc.InstantQuery(`node_load1`))
	resp.Load5m = round1(h.pc.InstantQuery(`node_load5`))
	resp.Load15m = round1(h.pc.InstantQuery(`node_load15`))
	resp.NetworkRx = round1(h.pc.InstantQuery(`rate(node_network_receive_bytes_total[5m]) * 8 / 1e6`))
	resp.NetworkTx = round1(h.pc.InstantQuery(`rate(node_network_transmit_bytes_total[5m]) * 8 / 1e6`))
	resp.SwapUse = round1(h.pc.InstantQuery(`node_memory_SwapTotal_bytes > 0 and (1 - node_memory_SwapFree_bytes / node_memory_SwapTotal_bytes) * 100`))
	resp.DiskIOUtil = round1(h.pc.InstantQuery(`rate(node_disk_io_time_seconds_total[5m]) * 100`))
	resp.TcpTimeWait = round1(h.pc.InstantQuery(`node_sockstat_TCP_tw`))
	resp.OomKills1h = round1(h.pc.InstantQuery(`increase(node_vmstat_oom_kill[1h])`))
	resp.InodeUse = round1(h.pc.InstantQuery(`(1 - node_filesystem_files_free{fstype!~"tmpfs|squashfs|overlay|fuse.*"} / node_filesystem_files{fstype!~"tmpfs|squashfs|overlay|fuse.*"}) * 100`))

	targets, err := h.pc.UpQuery()
	if err == nil {
		resp.Targets = targets
		for _, t := range targets {
			if t.Up {
				resp.UpCount++
			} else {
				resp.DownCount++
			}
		}
	}

	if resp.CPU == nil && resp.Memory == nil && resp.Disk == nil && resp.Load1m == nil {
		resp.Status = "error"
		resp.Error = "Prometheus 无响应或无可查询指标"
	}

	c.JSON(http.StatusOK, resp)
}

func round1(v *float64, _ error) *float64 {
	if v == nil {
		return nil
	}
	if math.IsNaN(*v) || math.IsInf(*v, 0) {
		return nil
	}
	rounded := float64(int(*v*10)) / 10
	return &rounded
}
