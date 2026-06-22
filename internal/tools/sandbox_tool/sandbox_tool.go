package sandboxtool

import (
	"context"
	"fmt"
	"sync"

	"github.com/LennyFace24/MiniAgent/internal/sandbox"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

// 存储每个会话的沙箱实例，key 为 sessionID，value 为 *sandbox.Sandbox
var sandboxes sync.Map

type SandBoxToolInput struct {
	Cmd         string `json:"cmd" description:"要在沙箱中执行的命令" required:"true"`
	Operation   string `json:"operation" description:"操作类型，可选值为 create、execute 或 destroy" required:"true"`
	ContainerID string `json:"container_id" description:"要操作的容器ID，仅在 execute 和 destroy 操作中需要" required:"false"`
}

func NewSandBoxTool(endpoint string) (tool.InvokableTool, error) {
	return utils.InferTool(
		"sandbox_tool",
		"用于创建和管理沙箱容器，支持创建、执行命令和销毁容器。",
		func(ctx context.Context, input SandBoxToolInput) (string, error) {
			switch input.Operation {
			case "create":
				temp_sandbox := sandbox.CreateContainer(endpoint)
				if temp_sandbox == nil {
					return "", fmt.Errorf("创建沙箱容器失败")
				}
				sandboxes.Store(temp_sandbox.ContainerID, temp_sandbox)
				return "容器创建完成,id是:" + temp_sandbox.ContainerID, nil
			case "execute":
				if input.ContainerID == "" {
					return "", fmt.Errorf("沙箱容器未创建或已销毁")
				}
				tmp, ok := sandboxes.Load(input.ContainerID)
				if !ok {
					return "", fmt.Errorf("沙箱容器未创建或已销毁")
				}
				response := tmp.(*sandbox.SandBox).Execute(input.Cmd)
				if response.Error != "" {
					return "", fmt.Errorf("执行命令失败: %s", response.Error)
				}

				fmt.Printf("在容器%s中执行命令: %s\n", input.ContainerID, input.Cmd)
				return response.Output, nil
			case "destroy":
				if input.ContainerID == "" {
					return "", fmt.Errorf("沙箱容器未创建或已销毁")
				}
				tmp, ok := sandboxes.Load(input.ContainerID)
				if !ok {
					return "", fmt.Errorf("沙箱容器未创建或已销毁")
				}
				err := tmp.(*sandbox.SandBox).Destroy()
				if err != nil {
					return "", fmt.Errorf("销毁容器失败: %v", err)
				}
				fmt.Printf("容器%s已销毁\n", input.ContainerID)
				return "容器已销毁", nil
			default:
				return "", fmt.Errorf("未知操作类型: %s", input.Operation)
			}
		},
	)
}
