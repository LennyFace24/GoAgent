package sandbox

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// 远程接口
// /api/containers/create
// /api/containers/execute
// s.endpoint+"/api/containers/"+s.containerID

type SandBox struct {
	Endpoint    string // 远程api地址
	ContainerID string // 容器id
}

// 容器对象
type Container struct {
	ID string `json:"id"`
}

// 远程容器执行命令请求
type ExecuteRequest struct {
	Cmd         string `json:"cmd"`
	ContainerID string `json:"container_id"`
}

// 远程容器执行命令结果
type ExecuteResponse struct {
	Output string `json:"output"`
	Error  string `json:"error"`
}

func NewSandBox(endpoint string) *SandBox {
	return &SandBox{
		Endpoint:    endpoint,
		ContainerID: "",
	}
}

// 创建容器
func CreateContainer(endpoint string) *SandBox {

	if endpoint == "" {
		fmt.Println("endpoint is empty")
		return nil
	}
	if endpoint[len(endpoint)-1] != '/' {
		endpoint += "/"
	}
	// client
	client := &http.Client{
		Timeout: 60 * time.Second,
	}

	req, err := http.NewRequest("POST", endpoint+"/api/containers/create", nil)
	if err != nil {
		fmt.Printf("创建请求失败: %v\n", err)
		return nil
	}

	res, err := client.Do(req)
	if err != nil {
		fmt.Printf("发送请求失败: %v\n", err)
		return nil
	}

	defer res.Body.Close()
	data, err := io.ReadAll(res.Body)
	if err != nil {
		fmt.Printf("读取响应失败: %v\n", err)
		return nil
	}
	var container Container
	err = json.Unmarshal(data, &container)
	if err != nil {
		fmt.Printf("解析响应失败: %v\n", err)
		return nil
	}
	// 创建容器，返回containerID
	return &SandBox{
		Endpoint:    endpoint,
		ContainerID: container.ID,
	}
}

// 执行操作
func (s *SandBox) Execute(cmd string) ExecuteResponse {
	if cmd == "" {
		return ExecuteResponse{Error: "cmd is empty"}
	}
	client := &http.Client{
		Timeout: 60 * time.Second,
	}
	var reqBody ExecuteRequest
	reqBody.Cmd = cmd
	reqBody.ContainerID = s.ContainerID

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return ExecuteResponse{Error: fmt.Sprintf("序列化请求失败: %v", err)}
	}
	req, err := http.NewRequest("POST", s.Endpoint+"/api/containers/execute", bytes.NewBuffer(jsonData))
	if err != nil {
		return ExecuteResponse{Error: fmt.Sprintf("创建请求失败: %v", err)}
	}

	res, err := client.Do(req)
	if err != nil {
		return ExecuteResponse{Error: fmt.Sprintf("发送请求失败: %v", err)}
	}
	defer res.Body.Close()

	data, err := io.ReadAll(res.Body)
	if err != nil {
		return ExecuteResponse{Error: fmt.Sprintf("发送请求失败: %v", err)}
	}

	var response ExecuteResponse
	err = json.Unmarshal(data, &response)
	if err != nil {
		return ExecuteResponse{Error: fmt.Sprintf("解析响应失败: %v", err)}
	}
	if response.Error != "" {
		return ExecuteResponse{Error: fmt.Sprintf("执行命令失败: %s", response.Error)}
	}
	fmt.Println("命令执行成功")

	return ExecuteResponse{Output: response.Output}

}

// 销毁容器
func (s *SandBox) Destroy() error {
	client := &http.Client{
		Timeout: 60 * time.Second,
	}
	req, err := http.NewRequest("DELETE", s.Endpoint+"/api/containers/"+s.ContainerID, nil)
	if err != nil {
		return fmt.Errorf("创建请求失败: %v", err)
	}

	res, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("发送请求失败: %v", err)
	}
	defer res.Body.Close()

	fmt.Printf("容器%s已销毁\n", s.ContainerID)
	s.ContainerID = ""
	return nil
}
