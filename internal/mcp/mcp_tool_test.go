package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/cloudwego/eino/components/tool"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

const bigIntegerLiteral = "9007199254740993"

// TestExportedToolNameIsModelSafeAndBounded checks the name handed to the model
// is always within the provider limit and uses only [A-Za-z0-9_-].
func TestExportedToolNameIsModelSafeAndBounded(t *testing.T) {
	cases := []struct{ server, tool string }{
		{"demo", "greet"},
		{"filesystem", "read_file"},
		{"my.server", "ns.tool.v2"},
		{"weird name!", "工具/名 称"},
		{"my_server", "read/value"},
		{"my_server", "read_value"},
		{"my.server", "read_value"},
		{"", ""},
		{strings.Repeat("s", 200), strings.Repeat("t", 200)},
		{"a__b", "c"},
		{"a", "b__c"},
		{"x", strings.Repeat("y", 120)},
	}

	seen := make(map[string]struct{ server, tool string })
	for _, tc := range cases {
		name := exportedToolName(tc.server, tc.tool)

		if len(name) > exportedNameMaxLen {
			t.Fatalf("exportedToolName(%q, %q) = %q, length %d exceeds %d",
				tc.server, tc.tool, name, len(name), exportedNameMaxLen)
		}
		if !isModelSafeName(name) {
			t.Fatalf("exportedToolName(%q, %q) = %q, contains characters outside [A-Za-z0-9_-]",
				tc.server, tc.tool, name)
		}
		if prev, ok := seen[name]; ok {
			t.Fatalf("exportedToolName(%q, %q) = %q collides with (%q, %q)",
				tc.server, tc.tool, name, prev.server, prev.tool)
		}
		seen[name] = struct{ server, tool string }{tc.server, tc.tool}
	}
}

// TestExportedToolNameSeparatorCollisions guards the tuple ambiguity that "__"
// inside server or tool names would otherwise create: ("a__b", "c") and
// ("a", "b__c") must not render as the same name.
func TestExportedToolNameSeparatorCollisions(t *testing.T) {
	first := exportedToolName("a__b", "c")
	second := exportedToolName("a", "b__c")
	if first == second {
		t.Fatalf("separator tuples collapsed to one name: %q", first)
	}
	if !isModelSafeName(first) || !isModelSafeName(second) {
		t.Fatalf("hashed names must stay model safe: %q, %q", first, second)
	}

	// Long names must not collide after truncation either.
	pairs := [][2][2]string{
		{{strings.Repeat("s", 60) + "A", "t"}, {strings.Repeat("s", 60) + "B", "t"}},
		{{"srv", strings.Repeat("t", 60) + "A"}, {"srv", strings.Repeat("t", 60) + "B"}},
	}
	for _, p := range pairs {
		if exportedToolName(p[0][0], p[0][1]) == exportedToolName(p[1][0], p[1][1]) {
			t.Fatalf("truncated names collide: %v vs %v", p[0], p[1])
		}
	}
}

func isModelSafeName(name string) bool {
	if name == "" {
		return false
	}
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_', r == '-':
		default:
			return false
		}
	}
	return true
}

// TestNewMCPToolRejectsBrokenInputSchema ensures a missing or non-object schema
// fails instead of being replaced by an empty object schema, which would tell
// the model that any argument is valid.
func TestNewMCPToolRejectsBrokenInputSchema(t *testing.T) {
	cases := []struct {
		name   string
		schema any
	}{
		{"missing", nil},
		{"null", json.RawMessage(`null`)},
		{"boolean true", json.RawMessage(`true`)},
		{"boolean false", json.RawMessage(`false`)},
		{"missing type", json.RawMessage(`{"properties":{}}`)},
		{"nullable root", json.RawMessage(`{"type":["object","null"]}`)},
		{"array", json.RawMessage(`["nope"]`)},
		{"string type", json.RawMessage(`{"type":"string"}`)},
		{"truncated json", json.RawMessage(`{"type":"object","properties":{`)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := newMCPTool(NewMCPManager(), "demo", &sdkmcp.Tool{Name: "greet", InputSchema: tc.schema}); err == nil {
				t.Fatalf("newMCPTool() accepted a %s input schema", tc.name)
			}
		})
	}
}

// TestNewMCPToolPreservesComplexSchema checks nested properties, required,
// $defs/$ref and union keywords survive the conversion to Eino's JSON schema.
func TestNewMCPToolPreservesComplexSchema(t *testing.T) {
	remote := &sdkmcp.Tool{
		Name: "configure",
		InputSchema: json.RawMessage(`{
			"$schema": "https://json-schema.org/draft/2020-12/schema",
			"type": "object",
			"required": ["target"],
			"properties": {
				"target": {"$ref": "#/$defs/Ref"},
				"mode": {"type": ["string", "null"], "enum": ["fast", "slow", null]},
				"retries": {"type": "integer", "minimum": 0}
			},
			"$defs": {"Ref": {"type": "object", "required": ["id"], "properties": {"id": {"type": "string"}}}}
		}`),
	}

	adapted, err := newMCPTool(NewMCPManager(), "demo", remote)
	if err != nil {
		t.Fatalf("newMCPTool() error: %v", err)
	}
	info, err := adapted.Info(context.Background())
	if err != nil {
		t.Fatalf("Info() error: %v", err)
	}
	converted, err := info.ParamsOneOf.ToJSONSchema()
	if err != nil {
		t.Fatalf("ToJSONSchema() error: %v", err)
	}
	if converted == nil {
		t.Fatal("ToJSONSchema() returned nil")
	}
	if converted.Type != "object" {
		t.Fatalf("schema type = %q, want object", converted.Type)
	}
	if len(converted.Required) != 1 || converted.Required[0] != "target" {
		t.Fatalf("required = %v, want [target]", converted.Required)
	}
	if converted.Definitions == nil || converted.Definitions["Ref"] == nil {
		t.Fatalf("$defs lost in conversion: %#v", converted.Definitions)
	}
	target, ok := converted.Properties.Get("target")
	if !ok || target == nil || target.Ref != "#/$defs/Ref" {
		t.Fatalf("target $ref lost in conversion: %#v", target)
	}
	mode, ok := converted.Properties.Get("mode")
	if !ok || mode == nil || len(mode.TypeEnhanced) != 2 || len(mode.Enum) != 3 {
		t.Fatalf("union/enum keywords lost in conversion: %#v", mode)
	}
	retries, ok := converted.Properties.Get("retries")
	if !ok || retries == nil || retries.Minimum.String() != "0" {
		t.Fatalf("numeric keyword lost in conversion: %#v", retries)
	}

}

// TestDecodeArgumentsKeepsLargeIntegerPrecision ensures integers beyond float64
// precision are not silently rounded on their way to the MCP server.
func TestDecodeArgumentsKeepsLargeIntegerPrecision(t *testing.T) {
	args, err := decodeArguments(`{"id":` + bigIntegerLiteral + `,"nested":{"seq":12345678901234567890}}`)
	if err != nil {
		t.Fatalf("decodeArguments() error: %v", err)
	}

	number, ok := args["id"].(json.Number)
	if !ok {
		t.Fatalf("id decoded as %T, want json.Number", args["id"])
	}
	if number.String() != bigIntegerLiteral {
		t.Fatalf("id = %s, want %s", number.String(), bigIntegerLiteral)
	}

	nested, ok := args["nested"].(map[string]any)
	if !ok {
		t.Fatalf("nested decoded as %T, want map[string]any", args["nested"])
	}
	if got := nested["seq"].(json.Number).String(); got != "12345678901234567890" {
		t.Fatalf("nested.seq = %s, want 12345678901234567890", got)
	}

	// The value must survive re-marshaling onto the wire unchanged.
	data, err := json.Marshal(args)
	if err != nil {
		t.Fatalf("Marshal() error: %v", err)
	}
	if !strings.Contains(string(data), bigIntegerLiteral) {
		t.Fatalf("marshaled arguments lost precision: %s", data)
	}
}

// TestDecodeArgumentsRejectsInvalidJSON blocks non-object and trailing payloads
// instead of silently dropping or partially applying them.
func TestDecodeArgumentsRejectsInvalidJSON(t *testing.T) {
	for _, raw := range []string{`[1,2]`, `null`, `"text"`, `{"a":1}garbage`, `{"a":}`, `{} {}`, `{} ]`, `{} 42`} {
		if _, err := decodeArguments(raw); err == nil {
			t.Fatalf("decodeArguments(%q) unexpectedly succeeded", raw)
		}
	}
	args, err := decodeArguments("  ")
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(args)
	if err != nil || string(data) != "{}" {
		t.Fatalf("omitted tool arguments = %s, %v; want empty JSON object", data, err)
	}
}

// TestRenderCallToolResultTextOnly keeps plain successful text as plain text.
func TestRenderCallToolResultTextOnly(t *testing.T) {
	result := &sdkmcp.CallToolResult{Content: []sdkmcp.Content{
		&sdkmcp.TextContent{Text: "hello"},
		&sdkmcp.TextContent{Text: "world"},
	}}

	got, err := renderCallToolResult("demo", "greet", result)
	if err != nil {
		t.Fatalf("renderCallToolResult() error: %v", err)
	}
	if got != "hello\nworld" {
		t.Fatalf("renderCallToolResult() = %q, want hello\\nworld", got)
	}
}

// TestRenderCallToolResultPreservesToolError keeps MCP tool-level failures
// visible to the model as data (never a Go error) and preserves isError.
func TestRenderCallToolResultPreservesToolError(t *testing.T) {
	result := &sdkmcp.CallToolResult{
		Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: "boom"}},
		IsError: true,
	}

	got, err := renderCallToolResult("demo", "greet", result)
	if err != nil {
		t.Fatalf("tool-level error must be data, got Go error: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal([]byte(got), &decoded); err != nil {
		t.Fatalf("tool error result is not JSON: %v (%q)", err, got)
	}
	if decoded["isError"] != true {
		t.Fatalf("isError lost: %v", decoded["isError"])
	}
	if _, ok := decoded["content"]; !ok {
		t.Fatalf("content lost: %v", decoded)
	}
}

// TestRenderCallToolResultPreservesStructuredAndMixedContent ensures structured
// and non-text output are not flattened away.
func TestRenderCallToolResultPreservesStructuredAndMixedContent(t *testing.T) {
	result := &sdkmcp.CallToolResult{
		Content:           []sdkmcp.Content{&sdkmcp.TextContent{Text: "summary"}, &sdkmcp.ImageContent{MIMEType: "image/png", Data: []byte("AA")}},
		StructuredContent: map[string]any{"ok": true, "count": 2},
	}

	got, err := renderCallToolResult("demo", "chart", result)
	if err != nil {
		t.Fatalf("renderCallToolResult() error: %v", err)
	}

	var decoded struct {
		Content           []map[string]any `json:"content"`
		StructuredContent map[string]any   `json:"structuredContent"`
	}
	if err := json.Unmarshal([]byte(got), &decoded); err != nil {
		t.Fatalf("result is not JSON: %v (%q)", err, got)
	}
	if len(decoded.Content) != 2 {
		t.Fatalf("content entries = %d, want 2 (%v)", len(decoded.Content), decoded.Content)
	}
	if decoded.Content[1]["type"] != "image" {
		t.Fatalf("non-text content lost: %v", decoded.Content[1])
	}
	if decoded.StructuredContent["ok"] != true || decoded.StructuredContent["count"] != float64(2) {
		t.Fatalf("structuredContent lost: %v", decoded.StructuredContent)
	}

	// Structured output alone must also be preserved.
	structuredOnly := &sdkmcp.CallToolResult{StructuredContent: map[string]any{"value": "x"}}
	got, err = renderCallToolResult("demo", "chart", structuredOnly)
	if err != nil {
		t.Fatalf("renderCallToolResult() error: %v", err)
	}
	if !strings.Contains(got, `"structuredContent"`) || !strings.Contains(got, `"value":"x"`) {
		t.Fatalf("structured only result lost fields: %q", got)
	}
}

// TestRenderCallToolResultNilIsError makes a missing result a Go error instead
// of a fake success message.
func TestRenderCallToolResultNilIsError(t *testing.T) {
	if _, err := renderCallToolResult("demo", "greet", nil); err == nil {
		t.Fatal("renderCallToolResult(nil) unexpectedly succeeded")
	}
}

func TestRenderCallToolResultReportsUnencodableOutput(t *testing.T) {
	result := &sdkmcp.CallToolResult{
		Content:           []sdkmcp.Content{&sdkmcp.TextContent{Text: "partial result"}},
		StructuredContent: map[string]any{"unsupported": make(chan int)},
	}
	if output, err := renderCallToolResult("demo", "chart", result); err == nil || output != "" {
		t.Fatalf("serialization failure silently returned partial output %q: %v", output, err)
	}
}

// TestMCPToolInvokableRunKeepsIntegerOverSession drives a real SDK session to
// prove the decoded argument reaches the remote tool without precision loss,
// and that routing uses the captured tuple rather than the exported name.
//
// The server registers a raw handler so arguments are read verbatim: the typed
// AddTool path re-marshals arguments through float64 inside the SDK and would
// round the integer regardless of what this adapter sends.
func TestMCPToolInvokableRunKeepsIntegerOverSession(t *testing.T) {
	const serverName = "demo server"

	clientTransport, serverTransport := sdkmcp.NewInMemoryTransports()
	server := sdkmcp.NewServer(&sdkmcp.Implementation{Name: "test-server", Version: "1.0.0"}, nil)
	server.AddTool(&sdkmcp.Tool{
		Name:        "echo_id",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"id":{"type":"integer"}}}`),
	}, func(_ context.Context, req *sdkmcp.CallToolRequest) (*sdkmcp.CallToolResult, error) {
		return &sdkmcp.CallToolResult{
			Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: string(req.Params.Arguments)}},
		}, nil
	})

	serverSession, err := server.Connect(context.Background(), serverTransport, nil)
	if err != nil {
		t.Fatalf("server.Connect() error: %v", err)
	}
	defer serverSession.Close()

	client := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "test-client", Version: "1.0.0"}, nil)
	clientSession, err := client.Connect(context.Background(), clientTransport, nil)
	if err != nil {
		t.Fatalf("client.Connect() error: %v", err)
	}
	defer clientSession.Close()

	manager := NewMCPManager()
	manager.servers[serverName] = clientSession

	adapted, err := manager.Tools(context.Background())
	if err != nil {
		t.Fatalf("Tools() error: %v", err)
	}
	if len(adapted) != 1 {
		t.Fatalf("Tools() returned %d tools, want 1", len(adapted))
	}

	info, err := adapted[0].Info(context.Background())
	if err != nil {
		t.Fatalf("Info() error: %v", err)
	}
	if !isModelSafeName(info.Name) || len(info.Name) > exportedNameMaxLen {
		t.Fatalf("exported name is not model safe/bounded: %q", info.Name)
	}
	if info.Extra["mcp_server"] != serverName || info.Extra["mcp_tool"] != "echo_id" {
		t.Fatalf("routing tuple lost: %#v", info.Extra)
	}

	invokable, ok := adapted[0].(interface {
		InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error)
	})
	if !ok {
		t.Fatal("adapted MCP tool does not implement InvokableRun")
	}

	got, err := invokable.InvokableRun(context.Background(), `{"id":`+bigIntegerLiteral+`}`)
	if err != nil {
		t.Fatalf("InvokableRun() error: %v", err)
	}
	if !strings.Contains(got, bigIntegerLiteral) {
		t.Fatalf("arguments reached the server with precision loss: %q", got)
	}
	if strings.Contains(got, "9007199254740992") {
		t.Fatalf("arguments were rounded to float64: %q", got)
	}
}

// TestMCPToolInvokableRunPropagatesProtocolError makes transport/protocol
// failures Go errors instead of tool output.
func TestMCPToolInvokableRunPropagatesProtocolError(t *testing.T) {
	clientTransport, serverTransport := sdkmcp.NewInMemoryTransports()
	server := sdkmcp.NewServer(&sdkmcp.Implementation{Name: "test-server", Version: "1.0.0"}, nil)
	serverSession, err := server.Connect(context.Background(), serverTransport, nil)
	if err != nil {
		t.Fatalf("server.Connect() error: %v", err)
	}
	defer serverSession.Close()

	client := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "test-client", Version: "1.0.0"}, nil)
	clientSession, err := client.Connect(context.Background(), clientTransport, nil)
	if err != nil {
		t.Fatalf("client.Connect() error: %v", err)
	}

	manager := NewMCPManager()
	manager.servers["demo"] = clientSession

	// A tool that the server does not expose produces an MCP protocol error.
	adapted, err := newMCPTool(manager, "demo", &sdkmcp.Tool{
		Name:        "missing",
		InputSchema: json.RawMessage(`{"type":"object"}`),
	})
	if err != nil {
		t.Fatalf("newMCPTool() error: %v", err)
	}
	if _, err := adapted.InvokableRun(context.Background(), `{}`); err == nil {
		_ = clientSession.Close()
		t.Fatal("InvokableRun() unexpectedly succeeded for an unknown remote tool")
	}
	if err := clientSession.Close(); err != nil {
		t.Fatalf("clientSession.Close() error: %v", err)
	}
}

// TestMCPToolInvokableRunReturnsToolErrorAsData drives a real session whose
// tool reports a tool-level failure: the model must receive the failure as tool
// output carrying isError, never as a Go error.
func TestMCPToolInvokableRunReturnsToolErrorAsData(t *testing.T) {
	clientTransport, serverTransport := sdkmcp.NewInMemoryTransports()
	server := sdkmcp.NewServer(&sdkmcp.Implementation{Name: "test-server", Version: "1.0.0"}, nil)
	server.AddTool(&sdkmcp.Tool{
		Name:        "failing",
		InputSchema: json.RawMessage(`{"type":"object"}`),
	}, func(_ context.Context, _ *sdkmcp.CallToolRequest) (*sdkmcp.CallToolResult, error) {
		return &sdkmcp.CallToolResult{
			Content:           []sdkmcp.Content{&sdkmcp.TextContent{Text: "disk full"}},
			StructuredContent: map[string]any{"code": "EIO"},
			IsError:           true,
		}, nil
	})

	serverSession, err := server.Connect(context.Background(), serverTransport, nil)
	if err != nil {
		t.Fatalf("server.Connect() error: %v", err)
	}
	defer serverSession.Close()

	client := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "test-client", Version: "1.0.0"}, nil)
	clientSession, err := client.Connect(context.Background(), clientTransport, nil)
	if err != nil {
		t.Fatalf("client.Connect() error: %v", err)
	}
	defer clientSession.Close()

	manager := NewMCPManager()
	manager.servers["demo"] = clientSession

	adapted, err := manager.Tools(context.Background())
	if err != nil {
		t.Fatalf("Tools() error: %v", err)
	}
	if len(adapted) != 1 {
		t.Fatalf("Tools() returned %d tools, want 1", len(adapted))
	}

	invokable, ok := adapted[0].(interface {
		InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error)
	})
	if !ok {
		t.Fatal("adapted MCP tool does not implement InvokableRun")
	}

	got, err := invokable.InvokableRun(context.Background(), `{}`)
	if err != nil {
		t.Fatalf("tool-level failure must reach the model as data, got error: %v", err)
	}

	var decoded struct {
		IsError           bool             `json:"isError"`
		Content           []map[string]any `json:"content"`
		StructuredContent map[string]any   `json:"structuredContent"`
	}
	if err := json.Unmarshal([]byte(got), &decoded); err != nil {
		t.Fatalf("tool failure is not JSON: %v (%q)", err, got)
	}
	if !decoded.IsError {
		t.Fatalf("isError lost end to end: %q", got)
	}
	if len(decoded.Content) != 1 || decoded.Content[0]["text"] != "disk full" {
		t.Fatalf("error content lost: %v", decoded.Content)
	}
	if decoded.StructuredContent["code"] != "EIO" {
		t.Fatalf("error structuredContent lost: %v", decoded.StructuredContent)
	}
}
