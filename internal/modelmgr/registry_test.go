package modelmgr

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeModelJSON(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "model.json")
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatalf("write model.json: %v", err)
	}
	return path
}

const validModelJSON = `{
  "default": "mimo",
  "models": [
    {"id": "mimo", "name": "MiMo", "base_url": "https://a.com/v1", "api_key": "k1", "context_window": 200000, "max_completion_tokens": 8192, "safety_margin_tokens": 4096},
    {"id": "qwen", "base_url": "https://b.com/v1", "api_key": "k2"}
  ]
}`

func TestLoadValid(t *testing.T) {
	r, err := Load(writeModelJSON(t, validModelJSON))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if r.DefaultID() != "mimo" {
		t.Fatalf("DefaultID = %q, want mimo", r.DefaultID())
	}
	if m, ok := r.Get("qwen"); !ok || m.Name != "qwen" {
		t.Fatalf("Get(qwen) = %+v, %v; want name fallback to id", m, ok)
	}
	if got := r.List(); len(got) != 2 {
		t.Fatalf("List len = %d, want 2", len(got))
	}
}

func TestLoadBudget(t *testing.T) {
	r, _ := Load(writeModelJSON(t, validModelJSON))
	m, _ := r.Get("mimo")
	if got := m.ContextBudgetTokens(); got != 187712 {
		t.Fatalf("ContextBudgetTokens = %d, want 187712", got)
	}
	// 未配置窗口的模型回退默认
	q, _ := r.Get("qwen")
	if got := q.ContextBudgetTokens(); got != DefaultContextWindow {
		t.Fatalf("default budget = %d, want %d", got, DefaultContextWindow)
	}
}

func TestLoadErrors(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{"invalid json", `{bad`, "解析失败"},
		{"empty models", `{"default":"a","models":[]}`, "非空 models"},
		{"missing id", `{"default":"a","models":[{"base_url":"u","api_key":"k"}]}`, "缺少必填字段: id"},
		{"missing base_url", `{"default":"a","models":[{"id":"a","api_key":"k"}]}`, "base_url"},
		{"missing api_key", `{"default":"a","models":[{"id":"a","base_url":"u"}]}`, "api_key"},
		{"duplicate id", `{"default":"a","models":[{"id":"a","base_url":"u","api_key":"k"},{"id":"a","base_url":"u","api_key":"k"}]}`, "重复 id"},
		{"missing default", `{"models":[{"id":"a","base_url":"u","api_key":"k"}]}`, "缺少必填字段: default"},
		{"default not in list", `{"default":"x","models":[{"id":"a","base_url":"u","api_key":"k"}]}`, "不在 models 列表中"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Load(writeModelJSON(t, tc.body))
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %q, want substring %q", err.Error(), tc.want)
			}
		})
	}
}

func TestLoadMissingFile(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "nope.json"))
	if err == nil || !strings.Contains(err.Error(), "读取 model.json 失败") {
		t.Fatalf("err = %v, want missing-file error", err)
	}
}

func TestModelSetSwitch(t *testing.T) {
	r, err := Load(writeModelJSON(t, validModelJSON))
	if err != nil {
		t.Fatal(err)
	}
	ms := NewModelSet(r, nil)
	if ms.CurrentID() != "mimo" {
		t.Fatalf("initial current = %q, want mimo", ms.CurrentID())
	}

	// 切换成功
	if err := ms.Switch(context.Background(), "qwen"); err != nil {
		t.Fatalf("Switch: %v", err)
	}
	if ms.CurrentID() != "qwen" {
		t.Fatalf("after switch current = %q, want qwen", ms.CurrentID())
	}

	// 切换不存在模型
	if err := ms.Switch(context.Background(), "nope"); err == nil {
		t.Fatal("Switch(nope) expected error")
	}
	// 失败后保持原模型
	if ms.CurrentID() != "qwen" {
		t.Fatalf("current after failed switch = %q, want qwen", ms.CurrentID())
	}
}

func TestModelSetList(t *testing.T) {
	r, _ := Load(writeModelJSON(t, validModelJSON))
	ms := NewModelSet(r, nil)
	if got := len(ms.ListModels()); got != 2 {
		t.Fatalf("ListModels len = %d, want 2", got)
	}
}

func TestLoadRecordsLoadError(t *testing.T) {
	LoadError = ""
	t.Cleanup(func() { LoadError = "" })

	path := writeModelJSON(t, `{bad`)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid json")
	}
	if LoadError == "" {
		t.Fatal("LoadError 应记录加载失败信息")
	}
	if !strings.Contains(LoadError, "解析失败") {
		t.Fatalf("LoadError = %q, want 解析失败", LoadError)
	}
}

func TestLoadClearsLoadErrorOnSuccess(t *testing.T) {
	LoadError = "stale error"
	t.Cleanup(func() { LoadError = "" })

	_, err := Load(writeModelJSON(t, validModelJSON))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if LoadError != "" {
		t.Fatalf("LoadError 应被清空，got %q", LoadError)
	}
}

func TestDefaultRegistry(t *testing.T) {
	r := DefaultRegistry()
	if r.DefaultID() != "default" {
		t.Fatalf("DefaultID = %q, want default", r.DefaultID())
	}
	if got := len(r.List()); got != 1 {
		t.Fatalf("List len = %d, want 1", got)
	}
}
