package main

import (
	"context"
	"fmt"
	"log"

	"github.com/LennyFace24/MiniAgent/internal/mcp"
)

func main() {
	// 1. 创建管理器并初始化
	fmt.Println("初始化 MCP 管理器...")
	manager := mcp.NewMCPManager()
	if err := manager.Init(); err != nil {
		log.Fatalf("初始化失败: %v", err)
	}

	// 2. 显示状态
	manager.PrintStatus()

	// 3. 连接所有启用的服务器
	ctx := context.Background()
	fmt.Println("连接启用的服务器...")
	if err := manager.ConnectAll(ctx); err != nil {
		log.Printf("部分连接失败: %v", err)
	}

	// 4. 再次显示状态
	manager.PrintStatus()

	// 5. 列出工具（如果有连接的服务器）
	sessions := manager.GetAllSessions()
	for name, session := range sessions {
		fmt.Printf("\n服务器 %s 的工具:\n", name)
		tools, err := manager.ListTools(ctx, name)
		if err != nil {
			log.Printf("获取工具失败: %v", err)
			continue
		}
		for _, t := range tools {
			fmt.Printf("  - %s: %s\n", t.Name, t.Description)
		}
		_ = session
	}

	// 6. 显示配置文件位置
	configPath, _ := mcp.GetMCPConfigPath()
	fmt.Printf("\n配置文件位置: %s\n", configPath)
	fmt.Println("\n提示: 编辑配置文件后，调用 manager.Reload() 重新加载配置")
}
