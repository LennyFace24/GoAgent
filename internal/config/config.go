package config

import (
	"fmt"
	"log"
	"os"
	"sync"

	"gopkg.in/yaml.v3"
)

var (
	config  *Config
	once    sync.Once
	loadErr error
)

type Config struct {
	Server struct {
		Host string `yaml:"host"`
		Port string `yaml:"port"`
	} `yaml:"server"`
	Llm struct {
		BaseUrl            string `yaml:"base_url"`
		ApiKey             string `yaml:"api_key"`
		Model              string `yaml:"model"`
		ContextWindow      int    `yaml:"context_window"`
		MaxTokens          int    `yaml:"max_completion_tokens"`
		SafetyMarginTokens int    `yaml:"safety_margin_tokens"`
	} `yaml:"llm"`
	Embedding struct {
		BaseUrl string `yaml:"base_url"`
		ApiKey  string `yaml:"api_key"`
		Model   string `yaml:"model"`
	} `yaml:"embedding"`
	ChromaDB struct {
		URL string `yaml:"url"`
	} `yaml:"chromadb"`
	Session struct {
		SecretKey string `yaml:"secret_key"`
	} `yaml:"session"`
	Workspace struct {
		Root string `yaml:"root"`
	} `yaml:"workspace"`
	Sandbox struct {
		Endpoint string `yaml:"endpoint"`
	} `yaml:"sandbox"`
}

const DefaultContextWindow = 200_000

func (c *Config) ApplyDefaults() {
	if c == nil {
		return
	}
	// 安全默认：未配置监听地址时只绑定本机回环，避免意外暴露到 0.0.0.0。
	// Docker 等需要对外监听的场景由配置文件显式覆盖（见 config.docker.yaml）。
	if c.Server.Host == "" {
		c.Server.Host = "127.0.0.1"
	}
	if c.Server.Port == "" {
		c.Server.Port = "8080"
	}
	if c.Llm.ContextWindow <= 0 {
		c.Llm.ContextWindow = DefaultContextWindow
	}
	if c.Llm.MaxTokens < 0 {
		c.Llm.MaxTokens = 0
	}
	if c.Llm.SafetyMarginTokens < 0 {
		c.Llm.SafetyMarginTokens = 0
	}
	if c.Workspace.Root == "" {
		c.Workspace.Root = "."
	}
	if c.Sandbox.Endpoint == "" {
		c.Sandbox.Endpoint = "http://localhost:8081"
	}
}

func (c *Config) ContextBudgetTokens() int64 {
	if c == nil {
		return DefaultContextWindow
	}

	c.ApplyDefaults()
	budget := c.Llm.ContextWindow - c.Llm.MaxTokens - c.Llm.SafetyMarginTokens
	if budget <= 0 {
		return int64(c.Llm.ContextWindow)
	}
	return int64(budget)
}

// Validate 校验必填字段，缺失时返回明确错误，避免延迟到 LLM 调用时才报错。
func (c *Config) Validate() error {
	if c == nil {
		return fmt.Errorf("config is nil")
	}
	if c.Llm.BaseUrl == "" {
		return fmt.Errorf("llm.base_url 不能为空")
	}
	if c.Llm.ApiKey == "" {
		return fmt.Errorf("llm.api_key 不能为空")
	}
	if c.Llm.Model == "" {
		return fmt.Errorf("llm.model 不能为空")
	}
	if c.Session.SecretKey == "" {
		return fmt.Errorf("session.secret_key 不能为空")
	}
	return nil
}

func LoadConfig(path string) (*Config, error) {
	once.Do(func() {
		config, loadErr = loadAndValidate(path)
	})
	return config, loadErr
}

func loadAndValidate(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}
	var c Config
	if err := yaml.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}
	c.ApplyDefaults()
	if err := c.Validate(); err != nil {
		return nil, err
	}
	return &c, nil
}

func GetConfig() *Config {
	if config == nil {
		log.Printf("配置文件未加载")
		return nil
	}
	return config
}
