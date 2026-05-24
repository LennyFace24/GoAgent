package task

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

type TaskPayLoad struct {
	Subject     string `json:"subject" description:"任务主题"`                                  // 任务主题
	Description string `json:"description" description:"任务描述"`                              // 任务描述
	Status      string `json:"status" description:"任务状态:pending | in_progress | completed"` // e.g., "pending", "in_progress", "completed"
	BlockBy     []int  `json:"block_by" description:"阻塞的任务"`
	Blocking    []int  `json:"blocking" description:"阻塞当前任务的任务"`
	Owner       string `json:"owner" description:"任务负责人"`
}

type Task struct {
	Id          int `json:"id"`
	TaskPayload TaskPayLoad
}

type UpdateInput struct {
	Id       int    `json:"id" description:"任务ID"`
	Status   string `json:"status" description:"新状态:pending | in_progress | completed"` // e.g., "pending", "in_progress", "completed"
	BlockBy  int    `json:"block_by" description:"新添加的阻塞的任务" omitempty:"true"`
	Blocking int    `json:"blocking" description:"新添加的阻塞当前任务的任务" omitempty:"true"`
}

const taskDir = ".tasks"

type TaskManager struct {
	mu sync.RWMutex
	id int // 自增id维护
}

func NewTaskManager() *TaskManager {
	if err := os.MkdirAll(taskDir, 0755); err != nil {
		panic("初始化 task 目录失败: " + err.Error())
	}
	return &TaskManager{}
}

// saveLocked 将 task 持久化到磁盘，调用方必须持有 t.mu 写锁
func (t *TaskManager) save(task Task) error {
	data, err := json.MarshalIndent(task, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal task: %w", err)
	}

	path := fmt.Sprintf(taskDir+"/task_%d.json", task.Id)
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to save task: %w", err)
	}
	return nil
}

func (t *TaskManager) CreateTaskTool() (tool.InvokableTool, error) {
	return utils.InferTool(
		"create_task",
		"创建一个新的任务，包含主题、描述和负责人,并持久化到.task目录下的一个文件中,taskId",
		func(ctx context.Context, taskPayLoad TaskPayLoad) (string, error) {
			// 解析成文件，保存到磁盘，内容为task的json格式
			// 名称为时间戳加随机字符串，确保唯一性
			t.mu.Lock()
			defer t.mu.Unlock()
			t.id++
			task := Task{
				Id:          t.id,
				TaskPayload: taskPayLoad,
			}
			err := t.save(task)
			if err != nil {
				return "", fmt.Errorf("failed to create task: %w", err)
			}
			return fmt.Sprintf("Task created with ID: %d", task.Id), nil
		},
	)
}

func (t *TaskManager) UpdateStatusTool() (tool.InvokableTool, error) {

	return utils.InferTool(
		"update_task_status",
		"更新任务状态，包括状态变更和阻塞关系的调整",
		func(ctx context.Context, input UpdateInput) (string, error) {
			// 存在taskId对应的任务，更新状态和阻塞关系，并持久化到磁盘
			var task Task

			t.mu.Lock()
			defer t.mu.Unlock()
			
			path := fmt.Sprintf(taskDir+"/task_%d.json", input.Id)	
			file,err := os.ReadFile(path)
			if err != nil {
				return "", fmt.Errorf("task with ID %d not found", input.Id)
			}
			json.Unmarshal(file, &task)
			if task.Id == input.Id {
				task.TaskPayload.Status = input.Status
				task.TaskPayload.BlockBy = append(task.TaskPayload.BlockBy, input.BlockBy)
				task.TaskPayload.Blocking = append(task.TaskPayload.Blocking, input.Blocking)
				err := t.save(task)
				if err != nil {
					return "", fmt.Errorf("failed to update task: %w", err)
				}
				return fmt.Sprintf("Task with ID %d updated successfully", input.Id), nil
			}
			return "", fmt.Errorf("task with ID %d not found", input.Id)
		},
	)

}

func (t *TaskManager) GetTaskTool() (tool.InvokableTool, error) {
	return utils.InferTool(
		"get_task",
		"获取任务详情，包括主题、描述、状态和阻塞关系",
		func(ctx context.Context, taskId int) (string, error) {
			t.mu.RLock()
			defer t.mu.RUnlock()

			path := fmt.Sprintf(taskDir+"/task_%d.json", taskId)
			jsonData, err := os.ReadFile(path)
			if err != nil {
				return fmt.Sprintf("Task with ID %d not found", taskId), nil
			}
			return string(jsonData), nil
		},
	)
}

func (t *TaskManager) ListTasksTool() (tool.InvokableTool, error) {
	return utils.InferTool(
		"list_tasks",
		"列出所有任务的基本信息，包括ID、主题和状态",
		func(ctx context.Context, _ struct{}) (string, error) {
			t.mu.RLock()
			defer t.mu.RUnlock()

			files, err := os.ReadDir(taskDir)
			if err != nil {
				return "", fmt.Errorf("failed to read tasks directory: %w", err)
			}
			var taskList []Task
			for _, file := range files {
				if file.IsDir() {
					continue
				}
				path := filepath.Join(taskDir, file.Name())
				jsonData, err := os.ReadFile(path)
				if err != nil {
					fmt.Printf("failed to read task file %s: %v\n", path, err)
					continue
				}
				var task Task
				err = json.Unmarshal(jsonData, &task)
				if err != nil {
					fmt.Printf("failed to unmarshal task file %s: %v\n", path, err)
					continue
				}
				taskList = append(taskList, task)
			}
			result, err := json.MarshalIndent(taskList, "", "  ")
			if err != nil {
				return "", fmt.Errorf("failed to marshal task list: %w", err)
			}
			return string(result), nil
		},
	)
}
