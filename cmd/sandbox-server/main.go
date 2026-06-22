package main

import (
	"os/exec"
	"strings"

	"github.com/LennyFace24/MiniAgent/internal/sandbox"

	"github.com/gin-gonic/gin"
)

// 启动沙箱服务器
func main() {

	// Sandbox Server 上跑一个定时清理 goroutine，把超过 1 小时没活动的容器删了就行。
	r := gin.Default()

	r.POST("/api/containers/create", handleCreate)
	r.POST("/api/containers/execute", handleExecute)
	r.DELETE("/api/containers/:id", handleDestroy)

	r.Run(":9090")
}

// 处理创建容器的请求
func handleCreate(c *gin.Context) {
	// 执行创建容器
	cmd := exec.Command("docker", "run", "-d", "--rm", "ubuntu", "sleep", "infinity")
	out, err := cmd.Output()

	// 获取容器id
	var containner sandbox.Container
	containner.ID = strings.TrimSpace(string(out))

	if err != nil {
		c.JSON(500, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(200, gin.H{
		"id": containner.ID,
	})
}

// 执行命令
func handleExecute(c *gin.Context) {

	// 请求体获取id
	var req sandbox.ExecuteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{
			"error": "请求参数错误",
		})
		return
	}
	// 执行命令
	cmd := exec.Command("docker", "exec", req.ContainerID, "sh", "-c", req.Cmd)
	// output() 改用 CombinedOutput()，以便同时获取标准输出和标准错误输出
	out, err := cmd.CombinedOutput()
	if err != nil {
		c.JSON(500, gin.H{
			"error": err.Error(),
		})
		return
	}
	c.JSON(200, gin.H{
		"output": string(out),
	})
}

// 销毁容器
func handleDestroy(c *gin.Context) {
	containerID := c.Param("id")
	cmd := exec.Command("docker", "rm", "-f", containerID)
	// output() 改用 CombinedOutput()，以便同时获取标准输出和标准错误输出
	log, err := cmd.CombinedOutput()
	if err != nil {
		c.JSON(500, gin.H{
			"error": err.Error(),
		})
		return
	}
	c.JSON(200, gin.H{
		"message": "容器已销毁,日志: " + string(log),
	})
}
