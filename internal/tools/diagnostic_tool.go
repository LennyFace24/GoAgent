package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

type HealthCheckInput struct {
	Service string `json:"service" description:"要检查的服务名称，可选" required:"false"`
}

type LogAnalysisInput struct {
	Service   string `json:"service" description:"服务名称" required:"true"`
	TimeRange string `json:"time_range" description:"时间范围，如 '14:00-14:30'" required:"false"`
	Keyword   string `json:"keyword" description:"日志关键词筛选" required:"false"`
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

func NewLogAnalyzerTool() (tool.InvokableTool, error) {
	return utils.InferTool(
		"log_analyzer",
		"分析指定服务的错误日志，返回错误模式、高频错误和异常事件",
		func(ctx context.Context, input LogAnalysisInput) (string, error) {
			return mockLogAnalysis(input.Service, input.TimeRange, input.Keyword), nil
		},
	)
}

type metricQuery struct {
	name  string
	query string
	unit  string
}

func queryPrometheus(baseURL, service string) (string, error) {
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
	url := fmt.Sprintf("%s/api/v1/query?query=%s", strings.TrimRight(baseURL, "/"), promQL)
	resp, err := client.Get(url)
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
		return "", nil
	}

	// 取第一条结果的 value[1]
	if len(pr.Data.Result) > 0 {
		if s, ok := pr.Data.Result[0].Value[1].(string); ok {
			return formatNumber(s), nil
		}
	}
	return "", nil
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

func mockLogAnalysis(service, timeRange, keyword string) string {
	if keyword == "" {
		keyword = "error"
	}
	return fmt.Sprintf(`[%s 日志分析]

时间范围: %s
关键词: %s

错误分布:
  timeout 错误: 234 条 (62%%)
  连接拒绝: 89 条 (23%%)
  500 内部错误: 45 条 (12%%)
  其他: 12 条 (3%%)

Top 3 错误信息:
1. dial tcp 10.0.1.5:3306: i/o timeout (出现 189 次)
2. connection pool exhausted, max=200, waiting=156 (出现 45 次)
3. slow query detected: SELECT * FROM orders WHERE ... (avg 4,200ms) (出现 23 次)

时间分布:
  14:00-14:10: ████████████ 89 条
  14:10-14:20: ████████████████████████ 156 条
  14:20-14:30: ██████████████████ 133 条`, service, timeRange, keyword)
}
