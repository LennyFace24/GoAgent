package extratool

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

type HealthCheckInput struct {
	Service string `json:"service" description:"要检查的服务名称，可选" required:"false"`
}


type prometheusResponse struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Metric map[string]string `json:"metric"`
			Value  [2]any            `json:"value"`
		} `json:"result"`
	} `json:"data"`
}

func NewHealthCheckTool(prometheusURL string) (tool.InvokableTool, error) {
	return utils.InferTool(
		"health_check",
		"检查系统健康状态，返回 CPU、内存、磁盘、负载等实时指标。可选 service 参数用于过滤特定 job。",
		func(ctx context.Context, input HealthCheckInput) (string, error) {
			if prometheusURL == "" {
				return "Prometheus 未配置，请在 config.yaml 中设置 prometheus.url", nil
			}
			return queryPrometheus(prometheusURL, input.Service)
		},
	)
}

type metricQuery struct {
	name  string
	query string
	unit  string
}

// health check tool
func queryPrometheus(baseURL string, service string) (string, error) {
	queries := []metricQuery{
		{"CPU 使用率", `100 - (avg(rate(node_cpu_seconds_total{mode="idle"}[5m])) * 100)`, "%"},
		{"内存使用率", `(1 - node_memory_MemAvailable_bytes / node_memory_MemTotal_bytes) * 100`, "%"},
		{"磁盘使用率 (/)", `(1 - node_filesystem_avail_bytes{mountpoint="/",fstype!="tmpfs"} / node_filesystem_size_bytes{mountpoint="/",fstype!="tmpfs"}) * 100`, "%"},
		{"系统负载 (1m)", `node_load1`, ""},
		{"系统负载 (5m)", `node_load5`, ""},
		{"系统负载 (15m)", `node_load15`, ""},
	}

	client := &http.Client{Timeout: 10 * time.Second}
	var results []string

	for _, mq := range queries {
		val, err := instantQuery(client, baseURL, mq.query)
		if err != nil {
			results = append(results, fmt.Sprintf("%s: 查询失败 (%v)", mq.name, err))
			continue
		}
		if val == "" {
			results = append(results, fmt.Sprintf("%s: 无数据", mq.name))
			continue
		}
		if mq.unit != "" {
			results = append(results, fmt.Sprintf("%s: %s%s", mq.name, val, mq.unit))
		} else {
			results = append(results, fmt.Sprintf("%s: %s", mq.name, val))
		}
	}

	// 额外查询 up 指标，看采集目标是否在线
	upResults, err := upQuery(client, baseURL)
	if err == nil && len(upResults) > 0 {
		results = append(results, "")
		results = append(results, "采集目标状态:")
		results = append(results, upResults...)
	}

	return fmt.Sprintf("[系统健康检查]\n\n%s", strings.Join(results, "\n")), nil
}

func instantQuery(client *http.Client, baseURL, promQL string) (string, error) {
	reqURL := fmt.Sprintf("%s/api/v1/query?query=%s", strings.TrimRight(baseURL, "/"), url.QueryEscape(promQL))
	resp, err := client.Get(reqURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var pr prometheusResponse
	if err := json.Unmarshal(body, &pr); err != nil {
		return "", err
	}
	if pr.Status != "success" {
		return "", fmt.Errorf("prometheus 返回非 success")
	}
	if len(pr.Data.Result) == 0 {
		return "(查询无结果)", nil
	}

	// 取第一条结果的 value[1]
	if len(pr.Data.Result) > 0 {
		if s, ok := pr.Data.Result[0].Value[1].(string); ok {
			return formatNumber(s), nil
		}
	}
	return "(查询结果格式异常)", nil
}

func upQuery(client *http.Client, baseURL string) ([]string, error) {
	url := fmt.Sprintf("%s/api/v1/query?query=up", strings.TrimRight(baseURL, "/"))
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var pr prometheusResponse
	if err := json.Unmarshal(body, &pr); err != nil {
		return nil, err
	}

	var lines []string
	for _, r := range pr.Data.Result {
		job := r.Metric["job"]
		instance := r.Metric["instance"]
		status := "在线"
		if s, ok := r.Value[1].(string); ok && s != "1" {
			status = "离线"
		}
		lines = append(lines, fmt.Sprintf("  %s (%s): %s", job, instance, status))
	}
	return lines, nil
}

func formatNumber(s string) string {
	// 截断到 2 位小数
	if idx := strings.Index(s, "."); idx != -1 && len(s) > idx+3 {
		return s[:idx+3]
	}
	return s
}

