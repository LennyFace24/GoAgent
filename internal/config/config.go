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
		BaseUrl   string `yaml:"base_url"`
		ApiKey    string `yaml:"api_key"`
		Model     string `yaml:"model"`
		MaxTokens int    `yaml:"max_completion_tokens"`
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
