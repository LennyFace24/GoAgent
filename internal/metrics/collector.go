package metrics

// Collector 系统指标采集接口
type Collector interface {
	Collect() (*MetricsSnapshot, error)
}

// MetricsSnapshot 系统指标快照，所有字段 *float64，nil 表示该指标不可用
type MetricsSnapshot struct {
	CPU        *float64 `json:"cpu,omitempty"`
	Memory     *float64 `json:"memory,omitempty"`
	MemTotalGB *float64 `json:"mem_total_gb,omitempty"`
	MemUsedGB  *float64 `json:"mem_used_gb,omitempty"`

	Disk      *float64 `json:"disk,omitempty"`
	InodeUse  *float64 `json:"inode_use,omitempty"`
	SwapUse   *float64 `json:"swap_use,omitempty"`
	DiskIOUtil *float64 `json:"disk_io_util,omitempty"`

	Load1m  *float64 `json:"load1m,omitempty"`
	Load5m  *float64 `json:"load5m,omitempty"`
	Load15m *float64 `json:"load15m,omitempty"`

	NetworkRx *float64 `json:"network_rx,omitempty"`
	NetworkTx *float64 `json:"network_tx,omitempty"`

	TcpTimeWait *float64 `json:"tcp_tw,omitempty"`
	OomKills1h  *float64 `json:"oom_kills_1h,omitempty"`
}

// TargetState 采集目标状态（兼容前端 targets 字段）
type TargetState struct {
	Job      string `json:"job"`
	Instance string `json:"instance"`
	Up       bool   `json:"up"`
}
