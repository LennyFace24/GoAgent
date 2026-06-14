package permission

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	basictool "github.com/LennyFace24/MiniAgent/internal/tools/basic_tool"
)



// ---------- 权限模式 ----------

type Mode string

const (
	ModeDefault Mode = "default" // 未命中规则时问用户
	ModePlan    Mode = "plan"    // 只允许读，不允许写
	ModeAuto    Mode = "auto"    // 读自动放行，写再问
)

var ValidModes = []Mode{ModeDefault, ModePlan, ModeAuto}

// ---------- 读写工具分类 ----------

var ReadTools = map[string]bool{
	"read_file":        true,
	"knowledge_search": true,
	"health_check":     true,
	"read_todo":        true,
	"skill":            true,
}

var WriteTools = map[string]bool{
	"write_file": true,
	"edit_file":  true,
	"bash":       true,
	"write_todo": true,
	"task":       true,
}

// ---------- 规则 ----------

type Behavior string

const (
	BehaviorAllow Behavior = "allow"
	BehaviorDeny  Behavior = "deny"
)

type Rule struct {
	Tool     string   // 工具名，"*" 匹配所有
	Path     string   // 路径 glob 模式，仅对 read_file/write_file/edit_file 生效
	Content  string   // 内容 glob 模式，仅对 bash 的 command 生效
	Behavior Behavior // allow 或 deny
}

// ---------- 决策结果 ----------

type Decision struct {
	Behavior Behavior // allow, deny, ask
	Reason   string
}

// ---------- 权限管理器 ----------

type PermissionManager struct {
	mu                  sync.RWMutex
	mode                Mode
	rules               []Rule
	consecutiveDenials  int
	maxConsecutiveDenials int
}

func NewPermissionManager(mode Mode) *PermissionManager {
	if !isValidMode(mode) {
		mode = ModeDefault
	}
	return &PermissionManager{
		mode:                  mode,
		rules:                 defaultRules(),
		maxConsecutiveDenials: 3,
	}
}

// ---------- 配置文件结构 ----------

type permissionFile struct {
	Permissions permissionRules `json:"permissions"`
}

type permissionRules struct {
	Allow       []string `json:"allow"`
	Ask         []string `json:"ask"`
	Deny        []string `json:"deny"`
	DefaultMode string   `json:"defaultMode"`
}

func loadPermissionConfig(path string) []Rule {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var file permissionFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil
	}

	var rules []Rule

	// deny 规则（优先级最高）
	for _, d := range file.Permissions.Deny {
		tool, content := parseBashPattern(d)
		rules = append(rules, Rule{Tool: tool, Content: content, Behavior: BehaviorDeny})
	}

	// ask 规则
	for _, a := range file.Permissions.Ask {
		tool, content := parseBashPattern(a)
		rules = append(rules, Rule{Tool: tool, Content: content, Behavior: "ask"})
	}

	// allow 规则
	for _, a := range file.Permissions.Allow {
		tool, content := parseBashPattern(a)
		rules = append(rules, Rule{Tool: tool, Content: content, Behavior: BehaviorAllow})
	}

	return rules
}

// parseBashPattern 解析 "Bash(command *)" 格式，返回 tool 和 content
// 例如: "Bash(ls *)" → tool="bash", content="ls *"
//       "read_file" → tool="read_file", content=""
func parseBashPattern(pattern string) (string, string) {
	if len(pattern) > 6 && pattern[:6] == "Bash(" && pattern[len(pattern)-1] == ')' {
		inner := pattern[6 : len(pattern)-1]
		return "bash", inner
	}
	return pattern, ""
}

func defaultRules() []Rule {
	// 从配置文件加载
	rules := loadPermissionConfig("permission.json")
	if len(rules) > 0 {
		return rules
	}
	// 回退到硬编码默认值
	return []Rule{
		{Tool: "bash", Content: "rm -rf /", Behavior: BehaviorDeny},
		{Tool: "bash", Content: "sudo *", Behavior: BehaviorDeny},
		{Tool: "read_file", Behavior: BehaviorAllow},
		{Tool: "write_file", Behavior: BehaviorAllow},
		{Tool: "edit_file", Behavior: BehaviorAllow},
		{Tool: "bash", Behavior: BehaviorAllow},
		{Tool: "knowledge_search", Behavior: BehaviorAllow},
		{Tool: "health_check", Behavior: BehaviorAllow},
		{Tool: "read_todo", Behavior: BehaviorAllow},
		{Tool: "write_todo", Behavior: BehaviorAllow},
		{Tool: "skill", Behavior: BehaviorAllow},
		{Tool: "task", Behavior: BehaviorAllow},
		{Tool: "subproxy", Behavior: BehaviorAllow},
		{Tool: "compact", Behavior: BehaviorAllow},
		{Tool: "context_status", Behavior: BehaviorAllow},
		{Tool: "create_task", Behavior: BehaviorAllow},
		{Tool: "update_task_status", Behavior: BehaviorAllow},
		{Tool: "get_task", Behavior: BehaviorAllow},
		{Tool: "list_tasks", Behavior: BehaviorAllow},
	}
}


// ---------- 核心：四阶段管道 ----------

func (pm *PermissionManager) Check(toolName string, toolInput map[string]any) Decision {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	// Step 0: Bash 安全校验（在 deny 规则之前）
	if toolName == "bash" {
		cmd, _ := toolInput["command"].(string)
		vr := basictool.ValidateBash(cmd)
		if len(vr.Failures) > 0 {
			if vr.Severe {
				return Decision{
					Behavior: BehaviorDeny,
					Reason:   fmt.Sprintf("Bash 安全校验拦截: %v", vr.Failures),
				}
			}
			// suspicious 级别：升级为 ask（由调用方处理）
			return Decision{
				Behavior: "ask",
				Reason:   fmt.Sprintf("Bash 安全校验标记: %v", vr.Failures),
			}
		}
	}

	// Step 1: Deny 规则（不可绕过）
	for _, rule := range pm.rules {
		if rule.Behavior != BehaviorDeny {
			continue
		}
		if pm.matches(rule, toolName, toolInput) {
			return Decision{
				Behavior: BehaviorDeny,
				Reason:   fmt.Sprintf("命中拒绝规则: tool=%s", rule.Tool),
			}
		}
	}

	// Step 2: Bash 只读判定
	if toolName == "bash" {
		cmd, _ := toolInput["command"].(string)
		if IsBashReadOnly(cmd) {
			return Decision{
				Behavior: BehaviorAllow,
				Reason:   "Bash 只读命令，自动放行",
			}
		}
		// 非只读 → 继续走模式检查 → allow 规则 → ask
	}

	// Step 3: 模式检查
	switch pm.mode {
	case ModePlan:
		if WriteTools[toolName] {
			return Decision{
				Behavior: BehaviorDeny,
				Reason:   "Plan 模式: 写操作被阻止",
			}
		}
		return Decision{
			Behavior: BehaviorAllow,
			Reason:   "Plan 模式: 只读操作放行",
		}

	case ModeAuto:
		if ReadTools[toolName] {
			return Decision{
				Behavior: BehaviorAllow,
				Reason:   "Auto 模式: 只读工具自动放行",
			}
		}
		// 写操作继续走 allow 规则 → ask
	}

	// Step 3: Allow 规则
	for _, rule := range pm.rules {
		if rule.Behavior != BehaviorAllow {
			continue
		}
		if pm.matches(rule, toolName, toolInput) {
			pm.consecutiveDenials = 0
			return Decision{
				Behavior: BehaviorAllow,
				Reason:   fmt.Sprintf("命中放行规则: tool=%s", rule.Tool),
			}
		}
	}

	// Step 4: 交给用户确认
	return Decision{
		Behavior: "ask",
		Reason:   fmt.Sprintf("无规则匹配 %s，需要用户确认", toolName),
	}
}

// ---------- 规则匹配 ----------

func (pm *PermissionManager) matches(rule Rule, toolName string, toolInput map[string]any) bool {
	// 工具名匹配
	if rule.Tool != "*" && rule.Tool != toolName {
		return false
	}
	// 路径 glob 匹配
	if rule.Path != "" && rule.Path != "*" {
		path, _ := toolInput["path"].(string)
		if matched, _ := filepath.Match(rule.Path, path); !matched {
			return false
		}
	}
	// 内容 glob 匹配（bash command）
	if rule.Content != "" && rule.Content != "*" {
		cmd, _ := toolInput["command"].(string)
		if matched, _ := filepath.Match(rule.Content, cmd); !matched {
			return false
		}
	}
	return true
}

// ---------- 拒绝计数 / 熔断器 ----------

func (pm *PermissionManager) RecordDenial() {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.consecutiveDenials++
}

func (pm *PermissionManager) ResetDenials() {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.consecutiveDenials = 0
}

func (pm *PermissionManager) ConsecutiveDenials() int {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.consecutiveDenials
}

func (pm *PermissionManager) ShouldSuggestPlanMode() bool {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.consecutiveDenials >= pm.maxConsecutiveDenials
}

// ---------- 运行时规则管理 ----------

func (pm *PermissionManager) AddRule(rule Rule) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.rules = append(pm.rules, rule)
}

func (pm *PermissionManager) SetMode(mode Mode) error {
	if !isValidMode(mode) {
		return fmt.Errorf("未知模式: %s，可选: %v", mode, ValidModes)
	}
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.mode = mode
	pm.consecutiveDenials = 0
	return nil
}

func (pm *PermissionManager) GetMode() Mode {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.mode
}

func (pm *PermissionManager) GetRules() []Rule {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	rules := make([]Rule, len(pm.rules))
	copy(rules, pm.rules)
	return rules
}

// ---------- 工具分类查询 ----------

func (pm *PermissionManager) IsReadOnly(toolName string) bool {
	return ReadTools[toolName] && !WriteTools[toolName]
}

func isValidMode(mode Mode) bool {
	for _, m := range ValidModes {
		if m == mode {
			return true
		}
	}
	return false
}
