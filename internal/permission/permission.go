package permission

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	basictool "github.com/LennyFace24/MiniAgent/internal/tools/basic_tool"
	"github.com/LennyFace24/MiniAgent/internal/util"
)

type Mode string

const (
	ModeDefault Mode = "default" // 未命中规则时询问用户
)

type Behavior string

const (
	BehaviorAllow Behavior = "allow"
	BehaviorAsk   Behavior = "ask"
	BehaviorDeny  Behavior = "deny"
)

type Rule struct {
	Tool     string
	Path     string
	Content  string
	Behavior Behavior
}

type Decision struct {
	Behavior Behavior
	Reason   string
}

type PermissionManager struct {
	mu                    sync.RWMutex
	mode                  Mode
	rules                 []Rule
	consecutiveDenials    int
	maxConsecutiveDenials int
}

func NewPermissionManager(mode Mode) *PermissionManager {
	return &PermissionManager{
		mode:                  ModeDefault,
		rules:                 loadPermissionConfig("permission.json"),
		maxConsecutiveDenials: 3,
	}
}

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
		panic("加载权限配置失败: " + err.Error())
	}

	var file permissionFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil
	}

	rules := make([]Rule, 0, len(file.Permissions.Deny)+len(file.Permissions.Ask)+len(file.Permissions.Allow))
	rules = appendRules(rules, file.Permissions.Deny, BehaviorDeny)
	rules = appendRules(rules, file.Permissions.Ask, BehaviorAsk)
	rules = appendRules(rules, file.Permissions.Allow, BehaviorAllow)
	return rules
}

func appendRules(rules []Rule, patterns []string, behavior Behavior) []Rule {
	for _, pattern := range patterns {
		tool, content := parseToolPattern(pattern)
		rules = append(rules, Rule{Tool: tool, Content: content, Behavior: behavior})
	}
	return rules
}

func parseToolPattern(pattern string) (string, string) {
	if strings.HasPrefix(pattern, "Bash(") && strings.HasSuffix(pattern, ")") {
		return "bash", strings.TrimSuffix(strings.TrimPrefix(pattern, "Bash("), ")")
	}
	return pattern, ""
}

func (pm *PermissionManager) Check(toolName string, toolInput map[string]any) Decision {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	if decision, ok := pm.validateBash(toolName, toolInput); ok {
		return decision
	}
	if decision, ok := pm.matchRule(toolName, toolInput, BehaviorDeny); ok {
		return decision
	}
	if decision, ok := pm.matchRule(toolName, toolInput, BehaviorAsk); ok {
		return decision
	}
	if decision, ok := pm.allowReadOnlyBash(toolName, toolInput); ok {
		return decision
	}
	if decision, ok := pm.matchRule(toolName, toolInput, BehaviorAllow); ok {
		pm.consecutiveDenials = 0
		return decision
	}

	return Decision{
		Behavior: BehaviorAsk,
		Reason:   fmt.Sprintf("无规则匹配 %s，需要用户确认", toolName),
	}
}

func (pm *PermissionManager) validateBash(toolName string, toolInput map[string]any) (Decision, bool) {
	if toolName != "bash" {
		return Decision{}, false
	}

	cmd, _ := toolInput["command"].(string)
	vr := basictool.ValidateBash(cmd)
	if len(vr.Failures) == 0 {
		return Decision{}, false
	}
	if vr.Severe {
		return Decision{
			Behavior: BehaviorDeny,
			Reason:   fmt.Sprintf("Bash 安全校验拦截: %v", vr.Failures),
		}, true
	}
	return Decision{
		Behavior: BehaviorAsk,
		Reason:   fmt.Sprintf("Bash 安全校验标记: %v", vr.Failures),
	}, true
}

func (pm *PermissionManager) allowReadOnlyBash(toolName string, toolInput map[string]any) (Decision, bool) {
	if toolName != "bash" {
		return Decision{}, false
	}

	cmd, _ := toolInput["command"].(string)
	if util.AnalyzeBashCommand(cmd) != util.BashAccessRead {
		return Decision{}, false
	}

	return Decision{
		Behavior: BehaviorAllow,
		Reason:   "AST 分析: 只读命令，自动放行",
	}, true
}

func (pm *PermissionManager) matchRule(toolName string, toolInput map[string]any, behavior Behavior) (Decision, bool) {
	for _, rule := range pm.rules {
		if rule.Behavior != behavior || !pm.matches(rule, toolName, toolInput) {
			continue
		}
		return Decision{
			Behavior: behavior,
			Reason:   fmt.Sprintf("命中%s规则: tool=%s", behavior, rule.Tool),
		}, true
	}
	return Decision{}, false
}

func (pm *PermissionManager) matches(rule Rule, toolName string, toolInput map[string]any) bool {
	if rule.Tool != "*" && rule.Tool != toolName {
		return false
	}
	if rule.Path != "" && rule.Path != "*" {
		path, _ := toolInput["path"].(string)
		if matched, _ := filepath.Match(rule.Path, path); !matched {
			return false
		}
	}
	if rule.Content != "" && rule.Content != "*" {
		cmd, _ := toolInput["command"].(string)
		return matchContent(rule.Content, cmd)
	}
	return true
}

func matchContent(pattern, value string) bool {
	if value == pattern {
		return true
	}
	if matched, _ := filepath.Match(pattern, value); matched {
		return true
	}
	if strings.HasSuffix(pattern, " *") {
		prefix := strings.TrimSuffix(pattern, " *")
		return value == prefix || strings.HasPrefix(value, prefix+" ")
	}
	return false
}

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

func (pm *PermissionManager) AddRule(rule Rule) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.rules = append(pm.rules, rule)
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
