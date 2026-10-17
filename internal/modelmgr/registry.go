package modelmgr

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
)

// modelFile 磁盘文件结构
type modelFile struct {
	DefaultID string  `json:"default"`
	Models    []Model `json:"models"`
}


// Registry 模型注册表：运行时从 model.json 加载。
// 只读共享：多服务可共享同一 Registry（进程级加载一次）；
// 每个服务通过 ModelSet 持有自己的激活模型（见 ModelSet）。
type Registry struct {
	models    map[string]Model
	defaultID string
}

// LoadError 记录最近一次加载失败的错误（供 /api/model 前端展示）
var LoadError string
// Load 加载并校验 model.json。格式错误返回分级明确的错误信息，
// 同时记录到 LoadError（前端 /api/model 可读取）。
func Load(path string) (*Registry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		LoadError = fmt.Sprintf("读取 model.json 失败: %v", err)
		return nil, fmt.Errorf("%s", LoadError)
	}

	var file modelFile
	if err := json.Unmarshal(data, &file); err != nil {
		LoadError = fmt.Sprintf("model.json 解析失败: %v", err)
		return nil, fmt.Errorf("%s", LoadError)
	}

	if len(file.Models) == 0 {
		LoadError = "model.json 必须包含非空 models 数组"
		return nil, fmt.Errorf("%s", LoadError)
	}

	reg := &Registry{models: make(map[string]Model, len(file.Models))}
	for i := range file.Models {
		m := file.Models[i]
		if m.ID == "" {
			LoadError = fmt.Sprintf("model.json 中 models[%d] 缺少必填字段: id", i)
			return nil, fmt.Errorf("%s", LoadError)
		}
		if m.BaseURL == "" {
			LoadError = fmt.Sprintf("model.json 中 models[%d] (id=%q) 缺少必填字段: base_url", i, m.ID)
			return nil, fmt.Errorf("%s", LoadError)
		}
		if m.APIKey == "" {
			LoadError = fmt.Sprintf("model.json 中 models[%d] (id=%q) 缺少必填字段: api_key", i, m.ID)
			return nil, fmt.Errorf("%s", LoadError)
		}
		if _, dup := reg.models[m.ID]; dup {
			LoadError = fmt.Sprintf("model.json 存在重复 id: %q", m.ID)
			return nil, fmt.Errorf("%s", LoadError)
		}
		if m.Name == "" {
			m.Name = m.ID
		}
		reg.models[m.ID] = m
	}

	if file.DefaultID == "" {
		LoadError = "model.json 缺少必填字段: default"
		return nil, fmt.Errorf("%s", LoadError)
	}
	if _, ok := reg.models[file.DefaultID]; !ok {
		LoadError = fmt.Sprintf("model.json 的 default 模型 %q 不在 models 列表中（可用: %s）", file.DefaultID, reg.IDs())
		return nil, fmt.Errorf("%s", LoadError)
	}
	reg.defaultID = file.DefaultID
	LoadError = "" // 加载成功清空错误
	return reg, nil
}

// List 返回全部模型（按 id 排序，稳定输出）
func (r *Registry) List() []Model {
	out := make([]Model, 0, len(r.models))
	for _, m := range r.models {
		out = append(out, m)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// Get 按 id 查询模型
func (r *Registry) Get(id string) (Model, bool) {
	m, ok := r.models[id]
	return m, ok
}

// Default 返回默认模型
func (r *Registry) Default() Model {
	return r.models[r.defaultID]
}

// DefaultID 返回默认模型 id
func (r *Registry) DefaultID() string {
	return r.defaultID
}

// IDs 返回模型 id 列表（校验错误信息用）
func (r *Registry) IDs() string {
	ids := make([]string, 0, len(r.models))
	for id := range r.models {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return strings.Join(ids, ", ")
}


// DefaultRegistry 返回内置默认模型（model.json 缺失/格式错误时降级使用，
// 保证服务可启动，前端通过 /api/model 的 error 字段看到问题）
func DefaultRegistry() *Registry {
	return &Registry{
		models: map[string]Model{
			"default": {
				ID:             "default",
				Name:           "默认模型（未配置 model.json）",
				BaseURL:        "",
				APIKey:         "",
				ContextWindow:  DefaultContextWindow,
				MaxCompletionTokens: 0,
				SafetyMarginTokens:  0,
			},
		},
		defaultID: "default",
	}
}
