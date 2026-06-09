package config

import (
	"fmt"
	"os"
	"sync"

	"gopkg.in/yaml.v3"
)

var (
	config *Config
	once   sync.Once
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
}

const DefaultContextWindow = 200_000

func (c *Config) ApplyDefaults() {
	if c == nil {
		return
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

func LoadConfig(path string) *Config {
	once.Do(func() {
		cfg, err := os.ReadFile(path)
		if err != nil {
			fmt.Errorf("读取配置文件出错: %v", err)
			return
		}
		err = yaml.Unmarshal(cfg, &config)
		if err != nil {
			fmt.Errorf("配置文件解析成yaml错误: %v", err)
			return
		}
		if config != nil {
			config.ApplyDefaults()
		}
	})
	return config
}

func GetConfig() *Config {
	if config == nil {
		fmt.Errorf("配置文件未加载")
		return nil
	}
	return config
}
