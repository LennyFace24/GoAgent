package extratool

import (
	"context"
	"fmt"
	"strings"

	"github.com/LennyFace24/MiniAgent/internal/util"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

type HealthCheckInput struct {
	Service string `json:"service" description:"要检查的服务名称，可选" required:"false"`
}

func NewHealthCheckTool(prometheusURL string) (tool.InvokableTool, error) {
	pc := util.NewPromClient(prometheusURL)

	return utils.InferTool(
		"health_check",
		"检查系统健康状态，返回 CPU、内存、磁盘、负载等实时指标。可选 service 参数用于过滤特定 job。",
		func(ctx context.Context, input HealthCheckInput) (string, error) {
			return queryPrometheus(pc)
		},
	)
}

type metricDef struct {
	name  string
	query string
	unit  string
}

func queryPrometheus(pc *util.PromClient) (string, error) {
	if pc.BaseURL == "" {
		return "Prometheus 未配置，请在 config.yaml 中设置 prometheus.url", nil
	}

	queries := []metricDef{
		{"CPU 使用率", `100 - (avg(rate(node_cpu_seconds_total{mode="idle"}[5m])) * 100)`, "%"},
		{"内存使用率", `(1 - node_memory_MemAvailable_bytes / node_memory_MemTotal_bytes) * 100`, "%"},
		{"磁盘使用率 (/)", `(1 - node_filesystem_avail_bytes{mountpoint="/",fstype!="tmpfs"} / node_filesystem_size_bytes{mountpoint="/",fstype!="tmpfs"}) * 100`, "%"},
		{"系统负载 (1m)", `node_load1`, ""},
		{"系统负载 (5m)", `node_load5`, ""},
		{"系统负载 (15m)", `node_load15`, ""},
	}

	var results []string
	for _, mq := range queries {
		v, err := pc.InstantQuery(mq.query)
		if err != nil {
			results = append(results, fmt.Sprintf("%s: 查询失败 (%v)", mq.name, err))
			continue
		}
		if v == nil {
			results = append(results, fmt.Sprintf("%s: 无数据", mq.name))
			continue
		}
		if mq.unit != "" {
			results = append(results, fmt.Sprintf("%s: %.1f%s", mq.name, *v, mq.unit))
		} else {
			results = append(results, fmt.Sprintf("%s: %.2f", mq.name, *v))
		}
	}

	targets, err := pc.UpQuery()
	if err == nil && len(targets) > 0 {
		results = append(results, "", "采集目标状态:")
		for _, t := range targets {
			status := "离线"
			if t.Up {
				status = "在线"
			}
			results = append(results, fmt.Sprintf("  %s (%s): %s", t.Job, t.Instance, status))
		}
	}

	return fmt.Sprintf("[系统健康检查]\n\n%s", strings.Join(results, "\n")), nil
}
