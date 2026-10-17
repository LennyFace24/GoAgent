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

// exportedNameMaxLen bounds the tool name handed to the model. Model providers
// reject long or exotic names, so anything that does not fit is truncated and
// disambiguated with a deterministic hash of the (server, tool) tuple.
const exportedNameMaxLen = 64

// mcpTool adapts one remote MCP tool to Eino's invokable tool interface.
//
// The exported name is namespaced so tools from different MCP servers cannot
// collide in the model's tool list. The original server and tool name are
// captured on the adapter, so a call never parses the exported name back apart.
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
			Name: name,
			Desc: description,
			Extra: map[string]any{
				"mcp_server": serverName,
				"mcp_tool":   remote.Name,
			},
			ParamsOneOf: schema.NewParamsOneOfByJSONSchema(inputSchema),
		},
	}, nil
}

// convertInputSchema turns a remote JSON Schema into Eino's representation.
//
// A missing or malformed schema is an error instead of an empty object schema:
// silently accepting one would tell the model that every argument is valid and
// hide the server's mistake until runtime.
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

// InvokableRun forwards JSON arguments to the selected MCP session and renders
// the MCP result as the string Eino's tool loop expects.
//
// A tool-level failure (CallToolResult.IsError) is data, not a Go error: the
// model has to see it to self-correct, including on the ask-approval path. Only
// protocol or marshaling failures become Go errors.
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

// decodeArguments parses the model's JSON arguments into MCP arguments.
//
// The decoder uses UseNumber so large integers survive the map[string]any round
// trip instead of being widened to float64 and losing precision. Non-object and
// trailing JSON are rejected rather than partially applied.
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

// renderCallToolResult renders an MCP result for the model.
//
// Successful text-only results stay plain joined text. Anything richer
// (structured content, non-text content) and any tool-level error is rendered
// as the full JSON of the result, so isError, content and structuredContent all
// reach the model instead of being flattened away.
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

// exportedToolName builds the model-visible name for a remote MCP tool.
//
// Simple names keep the readable "mcp__server__tool" form. Names that need
// sanitizing, that exceed exportedNameMaxLen, or whose server/tool boundary is
// ambiguous (they already contain the "__" separator) get a deterministic tuple
// hash instead. The manager rejects any remaining exported-name collision.
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

// hashedToolName builds a bounded, unambiguous name. The hash input is the
// length-prefixed (server, tool) tuple, so no concatenation of two different
// tuples can produce the same digest.
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

// sanitizeNamePart keeps only the characters allowed in model tool names:
// [A-Za-z0-9_-]. Every other rune collapses to '_'.
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
