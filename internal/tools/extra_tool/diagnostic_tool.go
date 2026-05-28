package extratool

import (
	"context"
	"fmt"
	"strings"

	"github.com/LennyFace24/MiniAgent/internal/metrics"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

type HealthCheckInput struct {
	Service string `json:"service" description:"要检查的服务名称，可选" required:"false"`
}

func NewHealthCheckTool(collector metrics.Collector) (tool.InvokableTool, error) {
	return utils.InferTool(
		"health_check",
		"检查系统健康状态，返回 CPU、内存、磁盘、负载等实时指标。可选 service 参数用于过滤特定 job。",
		func(ctx context.Context, input HealthCheckInput) (string, error) {
			return queryMetrics(collector)
		},
	)
}

type metricLine struct {
	name  string
	value *float64
	unit  string
}

func queryMetrics(collector metrics.Collector) (string, error) {
	snap, err := collector.Collect()
	if err != nil {
		return "指标采集失败: " + err.Error(), nil
	}

	lines := []metricLine{
		{"CPU 使用率", snap.CPU, "%"},
		{"内存使用率", snap.Memory, "%"},
		{"磁盘使用率", snap.Disk, "%"},
		{"系统负载 (1m)", snap.Load1m, ""},
		{"系统负载 (5m)", snap.Load5m, ""},
		{"系统负载 (15m)", snap.Load15m, ""},
	}

	var results []string
	for _, l := range lines {
		if l.value == nil {
			results = append(results, fmt.Sprintf("%s: 无数据", l.name))
			continue
		}
		if l.unit != "" {
			results = append(results, fmt.Sprintf("%s: %.1f%s", l.name, *l.value, l.unit))
		} else {
			results = append(results, fmt.Sprintf("%s: %.2f", l.name, *l.value))
		}
	}

	// 附加自身状态
	results = append(results, "", "采集目标状态:")
	results = append(results, fmt.Sprintf("  self (localhost): 在线"))

	return fmt.Sprintf("[系统健康检查]\n\n%s", strings.Join(results, "\n")), nil
}
