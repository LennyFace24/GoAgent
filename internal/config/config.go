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
	if c.Workspace.Root == "" {
		c.Workspace.Root = "."
	}
	if c.Sandbox.Endpoint == "" {
		c.Sandbox.Endpoint = "http://localhost:8081"
	}
}

// Validate 校验必填字段。LLM 配置已移至 model.json（见 internal/modelmgr），
// 此处不再校验 llm 段。
func (c *Config) Validate() error {
	if c == nil {
		return fmt.Errorf("config is nil")
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
