package main

import (
	"fmt"
	"log"

	"github.com/LennyFace24/MiniAgent/internal/mcp"
)

func main() {
	// 1. 初始化配置目录
	fmt.Println("初始化配置目录...")
	if err := mcp.InitConfigDir(); err != nil {
		log.Fatalf("初始化配置目录失败: %v", err)
	}

	// 2. 加载配置
	fmt.Println("加载 MCP 配置...")
	mcpConfig, err := mcp.LoadMCPConfig()
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	// 3. 显示配置信息
	fmt.Printf("配置版本: %s\n", mcpConfig.Version)
	fmt.Printf("服务器数量: %d\n", len(mcpConfig.Servers))

	// 4. 获取启用的服务器
	enabled := mcpConfig.GetEnabledServers()
	fmt.Printf("启用的服务器: %d\n", len(enabled))

	for name, server := range enabled {
		fmt.Printf("  - %s: %s (%s)\n", name, server.Description, server.Transport)
	}

	// 5. 使用管理器动态添加服务器
	fmt.Println("\n使用管理器添加新服务器...")
	manager := mcp.NewMCPManager()
	manager.LoadFromConfig(mcpConfig)

	newServer := &mcp.MCPServerConfig{
		Enabled:     true,
		Transport:   mcp.TransportStdio,
		Command:     "node",
		Args:        []string{"server.js"},
		Description: "动态添加的服务器",
	}

	if err := manager.AddServer("dynamic-server", newServer); err != nil {
		fmt.Printf("添加服务器失败: %v\n", err)
	} else {
		fmt.Println("成功添加服务器: dynamic-server")
	}

	// 6. 列出所有服务器
	fmt.Println("\n所有服务器:")
	for name, server := range manager.GetAllServers() {
		status := "禁用"
		if server.Enabled {
			status = "启用"
		}
		fmt.Printf("  - %s [%s]: %s\n", name, status, server.Description)
	}

	// 7. 显示配置文件位置
	configPath, _ := mcp.GetMCPConfigPath()
	fmt.Printf("\n配置文件位置: %s\n", configPath)
	fmt.Println("\n提示: 编辑配置文件后，重启程序或调用 Reload() 方法重新加载配置")
}
