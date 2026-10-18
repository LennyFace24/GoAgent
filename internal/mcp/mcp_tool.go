package mcp

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	einojsonschema "github.com/eino-contrib/jsonschema"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// exportedNameMaxLen 限制交给模型的工具名长度。模型提供方会拒绝过长或含特殊字符的名字，
// 因此放不下的名字会被截断，并用 (server, tool) 元组的确定性哈希消除歧义。
const exportedNameMaxLen = 64

// mcpTool 把一个远端 MCP 工具适配成 Eino 的可调用工具接口。
//
// 导出名做了命名空间隔离，避免不同 MCP 服务器的工具在模型工具列表里冲突。
// 原始 server/tool 名保存在适配器上，调用时不需要从导出名反解析。
type mcpTool struct {
	manager    *MCPManager
	serverName string
	remoteName string
	info       *schema.ToolInfo
}

func newMCPTool(manager *MCPManager, serverName string, remote *sdkmcp.Tool) (*mcpTool, error) {
	if remote == nil || strings.TrimSpace(remote.Name) == "" {
		return nil, fmt.Errorf("MCP server %q returned a tool without a name", serverName)
	}

	inputSchema, err := convertInputSchema(serverName, remote.Name, remote.InputSchema)
	if err != nil {
		return nil, err
	}

	name := exportedToolName(serverName, remote.Name)
	description := remote.Description
	if description == "" {
		description = fmt.Sprintf("MCP tool %s/%s", serverName, remote.Name)
	} else {
		description = fmt.Sprintf("MCP tool %s/%s: %s", serverName, remote.Name, description)
	}

	return &mcpTool{
		manager:    manager,
		serverName: serverName,
		remoteName: remote.Name,
		info: &schema.ToolInfo{
			Name:        name,
			Desc:        description,
			ParamsOneOf: schema.NewParamsOneOfByJSONSchema(inputSchema),
		},
	}, nil
}

// convertInputSchema 把远端 JSON Schema 转成 Eino 的表示。
//
// schema 缺失或格式错误会返回错误，而不是退化成空 object schema：
// 静默接受会让模型以为任意参数都合法，并把服务器的错误推迟到运行时才暴露。
func convertInputSchema(serverName, toolName string, remote any) (*einojsonschema.Schema, error) {
	if remote == nil {
		return nil, fmt.Errorf("MCP tool %s/%s has no input schema", serverName, toolName)
	}
	data, err := json.Marshal(remote)
	if err != nil {
		return nil, fmt.Errorf("marshal input schema for MCP tool %s/%s: %w", serverName, toolName, err)
	}

	if bytes.Equal(data, []byte("null")) {
		return nil, fmt.Errorf("MCP tool %s/%s has a null input schema", serverName, toolName)
	}

	var probe struct {
		Type any `json:"type"`
	}
	if err := json.Unmarshal(data, &probe); err != nil {
		return nil, fmt.Errorf("parse input schema for MCP tool %s/%s: %w", serverName, toolName, err)
	}
	if probe.Type != "object" {
		return nil, fmt.Errorf("MCP tool %s/%s has a non-object input schema (type=%v)", serverName, toolName, probe.Type)
	}

	converted := &einojsonschema.Schema{}
	if err := json.Unmarshal(data, converted); err != nil {
		return nil, fmt.Errorf("parse input schema for MCP tool %s/%s: %w", serverName, toolName, err)
	}
	return converted, nil
}

func (t *mcpTool) Info(context.Context) (*schema.ToolInfo, error) {
	return t.info, nil
}

// InvokableRun 把 JSON 参数转发给选定的 MCP 会话，并把结果渲染成 Eino 工具循环需要的字符串。
//
// 工具级失败（CallToolResult.IsError）是数据而不是 Go 错误：模型需要看到它才能自我纠正，
// 包括在权限确认路径上。只有协议或序列化失败才返回 Go 错误。
func (t *mcpTool) InvokableRun(ctx context.Context, argumentsInJSON string, _ ...tool.Option) (string, error) {
	args, err := decodeArguments(argumentsInJSON)
	if err != nil {
		return "", fmt.Errorf("MCP tool %s/%s arguments are invalid: %w", t.serverName, t.remoteName, err)
	}

	result, err := t.manager.CallTool(ctx, t.serverName, t.remoteName, args)
	if err != nil {
		return "", fmt.Errorf("call MCP tool %s/%s: %w", t.serverName, t.remoteName, err)
	}
	return renderCallToolResult(t.serverName, t.remoteName, result)
}

// decodeArguments 把模型的 JSON 参数解析成 MCP 参数。
//
// 使用 UseNumber 让大整数在 map[string]any 往返中保持精度，而不是被扩宽为 float64。
// 非对象和尾部多余 JSON 会被拒绝，而不是部分应用。
func decodeArguments(argumentsInJSON string) (map[string]any, error) {
	trimmed := strings.TrimSpace(argumentsInJSON)
	if trimmed == "" {
		return map[string]any{}, nil
	}

	dec := json.NewDecoder(strings.NewReader(trimmed))
	dec.UseNumber()
	var args map[string]any
	if err := dec.Decode(&args); err != nil {
		return nil, err
	}
	var trailing any
	if err := dec.Decode(&trailing); err != io.EOF {
		return nil, errors.New("unexpected trailing data after the JSON object")
	}
	if args == nil {
		return nil, errors.New("arguments must be a JSON object")
	}
	return args, nil
}

// renderCallToolResult 把 MCP 结果渲染给模型。
//
// 成功的纯文本结果保持简单拼接；更丰富的内容（结构化内容、非文本内容）以及工具级错误
// 都按完整 JSON 渲染，使 isError、content、structuredContent 都能到达模型，而不是被扁平化。
func renderCallToolResult(serverName, toolName string, result *sdkmcp.CallToolResult) (string, error) {
	if result == nil {
		return "", fmt.Errorf("MCP tool %s/%s returned no result", serverName, toolName)
	}

	if !result.IsError && result.StructuredContent == nil && len(result.Content) > 0 && allTextContent(result.Content) {
		texts := make([]string, 0, len(result.Content))
		for _, content := range result.Content {
			texts = append(texts, content.(*sdkmcp.TextContent).Text)
		}
		if joined := strings.Join(texts, "\n"); joined != "" {
			return joined, nil
		}
	}

	data, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("marshal result of MCP tool %s/%s: %w", serverName, toolName, err)
	}
	return string(data), nil
}

func allTextContent(contents []sdkmcp.Content) bool {
	for _, content := range contents {
		if _, ok := content.(*sdkmcp.TextContent); !ok {
			return false
		}
	}
	return true
}

// exportedToolName 生成远端 MCP 工具对模型可见的名字。
//
// 简单名字保留可读的 "mcp__server__tool" 形式。需要清洗、超过 exportedNameMaxLen，
// 或 server/tool 边界有歧义（本身已包含 "__" 分隔符）的名字，改用确定性元组哈希。
// 管理器会拒绝任何仍然存在的导出名冲突。
func exportedToolName(serverName, toolName string) string {
	server := sanitizeNamePart(serverName)
	tool := sanitizeNamePart(toolName)

	plain := "mcp__" + server + "__" + tool
	if len(plain) <= exportedNameMaxLen &&
		!strings.Contains(serverName, "__") && !strings.Contains(toolName, "__") &&
		server == serverName && tool == toolName {
		return plain
	}
	return hashedToolName(serverName, toolName)
}

// hashedToolName 生成有长度上限且无歧义的名字。哈希输入是带长度前缀的 (server, tool)
// 元组，因此不同元组拼接后不可能产生相同的摘要。
func hashedToolName(serverName, toolName string) string {
	digest := sha256.Sum256([]byte(
		strconv.Itoa(len(serverName)) + ":" + serverName + "|" +
			strconv.Itoa(len(toolName)) + ":" + toolName,
	))
	suffix := hex.EncodeToString(digest[:])[:12]

	const prefix = "mcp_"
	budget := exportedNameMaxLen - len(prefix) - len(suffix) - 2
	server := sanitizeNamePart(serverName)
	if len(server) > budget/2 {
		server = server[:budget/2]
	}
	remote := sanitizeNamePart(toolName)
	if len(remote) > budget-len(server) {
		remote = remote[:budget-len(server)]
	}
	return prefix + server + "_" + remote + "_" + suffix
}

// sanitizeNamePart 只保留模型工具名允许的字符：[A-Za-z0-9_-]，
// 其他字符统一替换为 '_'。
func sanitizeNamePart(value string) string {
	var b strings.Builder
	b.Grow(len(value))
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_', r == '-':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	if b.Len() == 0 {
		return "unknown"
	}
	return b.String()
}
