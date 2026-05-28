package handler

import (
	"net/http"

	"github.com/LennyFace24/MiniAgent/internal/metrics"
	"github.com/gin-gonic/gin"
)

type MetricsResponse struct {
	Status       string               `json:"status"`
	CPU          *float64             `json:"cpu,omitempty"`
	Memory       *float64             `json:"memory,omitempty"`
	MemTotalGB   *float64             `json:"mem_total_gb,omitempty"`
	MemUsedGB    *float64             `json:"mem_used_gb,omitempty"`
	Disk         *float64             `json:"disk,omitempty"`
	DiskIOUtil   *float64             `json:"disk_io_util,omitempty"`
	InodeUse     *float64             `json:"inode_use,omitempty"`
	SwapUse      *float64             `json:"swap_use,omitempty"`
	Load1m       *float64             `json:"load1m,omitempty"`
	Load5m       *float64             `json:"load5m,omitempty"`
	Load15m      *float64             `json:"load15m,omitempty"`
	NetworkRx    *float64             `json:"network_rx,omitempty"`
	NetworkTx    *float64             `json:"network_tx,omitempty"`
	TcpTimeWait  *float64             `json:"tcp_tw,omitempty"`
	OomKills1h   *float64             `json:"oom_kills_1h,omitempty"`
	UpCount      int                  `json:"up_count"`
	DownCount    int                  `json:"down_count"`
	Targets      []metrics.TargetState `json:"targets,omitempty"`
	Error        string               `json:"error,omitempty"`
}

type MetricsHandler struct {
	collector metrics.Collector
}

func NewMetricsHandler(collector metrics.Collector) *MetricsHandler {
	return &MetricsHandler{collector: collector}
}

func (h *MetricsHandler) GetMetrics(c *gin.Context) {
	snap, err := h.collector.Collect()
	if err != nil {
		c.JSON(http.StatusOK, MetricsResponse{
			Status: "error",
			Error:  "指标采集失败: " + err.Error(),
		})
		return
	}

	resp := MetricsResponse{
		Status: "ok",
		// 固定返回自身作为 target（不再依赖 Prometheus target 状态）
		UpCount:   1,
		DownCount: 0,
		Targets: []metrics.TargetState{
			{Job: "self", Instance: "localhost", Up: true},
		},
	}

	resp.CPU = snap.CPU
	resp.Memory = snap.Memory
	resp.MemTotalGB = snap.MemTotalGB
	resp.MemUsedGB = snap.MemUsedGB
	resp.Disk = snap.Disk
	resp.DiskIOUtil = snap.DiskIOUtil
	resp.InodeUse = snap.InodeUse
	resp.SwapUse = snap.SwapUse
	resp.Load1m = snap.Load1m
	resp.Load5m = snap.Load5m
	resp.Load15m = snap.Load15m
	resp.NetworkRx = snap.NetworkRx
	resp.NetworkTx = snap.NetworkTx
	resp.TcpTimeWait = snap.TcpTimeWait
	resp.OomKills1h = snap.OomKills1h

	c.JSON(http.StatusOK, resp)
}
