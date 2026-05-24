package handler

import (
	"net/http"

	"github.com/LennyFace24/MiniAgent/internal/util"
	"github.com/gin-gonic/gin"
)

type MetricsResponse struct {
	Status    string              `json:"status"`
	CPU       *float64            `json:"cpu,omitempty"`
	Memory    *float64            `json:"memory,omitempty"`
	Disk      *float64            `json:"disk,omitempty"`
	Load1m    *float64            `json:"load1m,omitempty"`
	Load5m    *float64            `json:"load5m,omitempty"`
	Load15m   *float64            `json:"load15m,omitempty"`
	NetworkRx *float64            `json:"network_rx,omitempty"`
	NetworkTx *float64            `json:"network_tx,omitempty"`
	UpCount   int                 `json:"up_count"`
	DownCount int                 `json:"down_count"`
	Targets   []util.TargetState  `json:"targets,omitempty"`
	Error     string              `json:"error,omitempty"`
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
	resp.Disk = round1(h.pc.InstantQuery(`(1 - node_filesystem_avail_bytes{mountpoint="/",fstype!="tmpfs"} / node_filesystem_size_bytes{mountpoint="/",fstype!="tmpfs"}) * 100`))
	resp.Load1m = round1(h.pc.InstantQuery(`node_load1`))
	resp.Load5m = round1(h.pc.InstantQuery(`node_load5`))
	resp.Load15m = round1(h.pc.InstantQuery(`node_load15`))
	resp.NetworkRx = round1(h.pc.InstantQuery(`rate(node_network_receive_bytes_total[5m]) * 8 / 1e6`))
	resp.NetworkTx = round1(h.pc.InstantQuery(`rate(node_network_transmit_bytes_total[5m]) * 8 / 1e6`))

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
	rounded := float64(int(*v*10)) / 10
	return &rounded
}
