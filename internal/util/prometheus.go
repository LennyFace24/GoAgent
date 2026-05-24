package util

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type PromClient struct {
	BaseURL    string
	HTTPClient *http.Client
}

func NewPromClient(baseURL string) *PromClient {
	return &PromClient{
		BaseURL:    baseURL,
		HTTPClient: &http.Client{Timeout: 10 * time.Second},
	}
}

type promResponse struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Metric map[string]string `json:"metric"`
			Value  [2]any            `json:"value"`
		} `json:"result"`
	} `json:"data"`
}

type TargetState struct {
	Job      string `json:"job"`
	Instance string `json:"instance"`
	Up       bool   `json:"up"`
}

func (pc *PromClient) InstantQuery(promQL string) (*float64, error) {
	if pc.BaseURL == "" {
		return nil, fmt.Errorf("Prometheus 未配置")
	}

	reqURL := fmt.Sprintf("%s/api/v1/query?query=%s",
		strings.TrimRight(pc.BaseURL, "/"), url.QueryEscape(promQL))

	resp, err := pc.HTTPClient.Get(reqURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var pr promResponse
	if err := json.Unmarshal(body, &pr); err != nil {
		return nil, err
	}
	if pr.Status != "success" || len(pr.Data.Result) == 0 {
		return nil, fmt.Errorf("无数据")
	}

	s, ok := pr.Data.Result[0].Value[1].(string)
	if !ok {
		return nil, fmt.Errorf("格式异常")
	}

	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return nil, err
	}
	return &v, nil
}

func (pc *PromClient) UpQuery() ([]TargetState, error) {
	reqURL := fmt.Sprintf("%s/api/v1/query?query=up",
		strings.TrimRight(pc.BaseURL, "/"))

	resp, err := pc.HTTPClient.Get(reqURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var pr promResponse
	if err := json.Unmarshal(body, &pr); err != nil {
		return nil, err
	}
	if pr.Status != "success" {
		return nil, fmt.Errorf("prometheus status: %s", pr.Status)
	}

	var targets []TargetState
	for _, r := range pr.Data.Result {
		up := false
		if s, ok := r.Value[1].(string); ok && s == "1" {
			up = true
		}
		targets = append(targets, TargetState{
			Job:      r.Metric["job"],
			Instance: r.Metric["instance"],
			Up:       up,
		})
	}
	return targets, nil
}
