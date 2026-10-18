package permission

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	pathpkg "path"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	basictool "github.com/LennyFace24/MiniAgent/internal/tools/basic_tool"
	"github.com/LennyFace24/MiniAgent/internal/util"
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
	rules                 []Rule
	scopedRules           map[string][]Rule
	consecutiveDenials    map[string]int
	maxConsecutiveDenials int
}

func NewPermissionManager() *PermissionManager {
	return &PermissionManager{
		rules:                 loadPermissionConfig("permission.json"),
		scopedRules:           make(map[string][]Rule),
		consecutiveDenials:    make(map[string]int),
		maxConsecutiveDenials: 3,
	}
}

type permissionFile struct {
	Permissions permissionRules `json:"permissions"`
}

type permissionRules struct {
	Allow []string `json:"allow"`
	Ask   []string `json:"ask"`
	Deny  []string `json:"deny"`
}

var persistMu sync.Mutex

// ScopeKey creates the in-memory scope used for one conversation.
func ScopeKey(sessionID, conversationID string) string {
	return sessionID + "\x00" + conversationID
}

func loadPermissionConfig(path string) []Rule {
	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("读取权限配置失败: %v", err)
		}
		return nil
	}

	var file permissionFile
	if err := json.Unmarshal(data, &file); err != nil {
		log.Printf("解析权限配置失败: %v", err)
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
		toolName, pathPattern, content := parseToolPattern(pattern)
		rules = append(rules, Rule{Tool: toolName, Path: pathPattern, Content: content, Behavior: behavior})
	}
	return rules
}

func parseToolPattern(pattern string) (toolName, pathPattern, content string) {
	if strings.HasPrefix(pattern, "Bash(") && strings.HasSuffix(pattern, ")") {
		return "bash", "", strings.TrimSuffix(strings.TrimPrefix(pattern, "Bash("), ")")
	}
	if open := strings.IndexByte(pattern, '('); open > 0 && strings.HasSuffix(pattern, ")") {
		return pattern[:open], strings.TrimSuffix(pattern[open+1:], ")"), ""
	}
	return pattern, "", ""
}

// RuleForInvocation creates a least-privilege allow rule for one approved call.
func RuleForInvocation(toolName string, toolInput map[string]any) Rule {
	rule := Rule{Tool: toolName, Behavior: BehaviorAllow}
	if toolName == "bash" {
		rule.Content, _ = toolInput["command"].(string)
		return rule
	}
	if path, ok := toolInput["path"].(string); ok && path != "" {
		rule.Path = filepath.ToSlash(filepath.Clean(path))
	}
	return rule
}

func (pm *PermissionManager) Check(scope, toolName string, toolInput map[string]any) Decision {
	if decision, ok := pm.validateBash(toolName, toolInput); ok {
		return decision
	}

	pm.mu.RLock()
	global := pm.rules
	scoped := pm.scopedRules[scope]
	if decision, ok := matchRules(global, scoped, toolName, toolInput, BehaviorDeny); ok {
		pm.mu.RUnlock()
		return decision
	}
	if decision, ok := matchRules(global, scoped, toolName, toolInput, BehaviorAsk); ok {
		pm.mu.RUnlock()
		return decision
	}
	pm.mu.RUnlock()

	if decision, ok := pm.allowReadOnlyBash(toolName, toolInput); ok {
		return decision
	}

	pm.mu.RLock()
	if decision, ok := matchRules(scoped, global, toolName, toolInput, BehaviorAllow); ok {
		pm.mu.RUnlock()
		return decision
	}
	pm.mu.RUnlock()

	return Decision{Behavior: BehaviorAsk, Reason: fmt.Sprintf("无规则匹配 %s，需要用户确认", toolName)}
}

func matchRules(primary, secondary []Rule, toolName string, toolInput map[string]any, behavior Behavior) (Decision, bool) {
	for _, rules := range [][]Rule{primary, secondary} {
		for _, rule := range rules {
			if rule.Behavior != behavior || !matches(rule, toolName, toolInput) {
				continue
			}
			return Decision{Behavior: behavior, Reason: fmt.Sprintf("命中%s规则: tool=%s", behavior, rule.Tool)}, true
		}
	}
	return Decision{}, false
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
		return Decision{Behavior: BehaviorDeny, Reason: fmt.Sprintf("Bash 安全校验拦截: %v", vr.Failures)}, true
	}
	return Decision{Behavior: BehaviorAsk, Reason: fmt.Sprintf("Bash 安全校验标记: %v", vr.Failures)}, true
}

func (pm *PermissionManager) allowReadOnlyBash(toolName string, toolInput map[string]any) (Decision, bool) {
	if toolName != "bash" {
		return Decision{}, false
	}
	cmd, _ := toolInput["command"].(string)
	if util.AnalyzeBashCommand(cmd) != util.BashAccessRead {
		return Decision{}, false
	}
	return Decision{Behavior: BehaviorAllow, Reason: "AST 分析: 只读命令，自动放行"}, true
}

func matches(rule Rule, toolName string, toolInput map[string]any) bool {
	if rule.Tool != "*" && rule.Tool != toolName {
		return false
	}
	if rule.Path != "" && rule.Path != "*" {
		path, _ := toolInput["path"].(string)
		if !matchPath(rule.Path, path) {
			return false
		}
	}
	if rule.Content != "" && rule.Content != "*" {
		cmd, _ := toolInput["command"].(string)
		if !matchContent(rule.Content, cmd) {
			return false
		}
	}
	return true
}

func matchPath(pattern, value string) bool {
	pattern = filepath.ToSlash(filepath.Clean(pattern))
	value = filepath.ToSlash(filepath.Clean(value))
	if strings.Contains(pattern, "**") {
		matched, err := regexp.MatchString(globRegex(pattern), value)
		return err == nil && matched
	}
	matched, err := pathpkg.Match(pattern, value)
	return err == nil && matched
}

func globRegex(pattern string) string {
	var b strings.Builder
	b.WriteByte('^')
	for i := 0; i < len(pattern); {
		switch pattern[i] {
		case '*':
			if i+1 < len(pattern) && pattern[i+1] == '*' {
				i += 2
				if i < len(pattern) && pattern[i] == '/' {
					i++
					b.WriteString(`(?:.*/)?`)
				} else {
					b.WriteString(`.*`)
				}
			} else {
				i++
				b.WriteString(`[^/]*`)
			}
		case '?':
			i++
			b.WriteString(`[^/]`)
		default:
			b.WriteString(regexp.QuoteMeta(string(pattern[i])))
			i++
		}
	}
	b.WriteByte('$')
	return b.String()
}

func matchContent(pattern, value string) bool {
	if value == pattern {
		return true
	}
	if matched, _ := pathpkg.Match(pattern, value); matched {
		return true
	}
	if strings.HasSuffix(pattern, " *") {
		prefix := strings.TrimSuffix(pattern, " *")
		return value == prefix || strings.HasPrefix(value, prefix+" ")
	}
	return false
}

func (pm *PermissionManager) RecordDenial(scope string) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.consecutiveDenials[scope]++
}

func (pm *PermissionManager) ResetDenials(scope string) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	delete(pm.consecutiveDenials, scope)
}

func (pm *PermissionManager) ConsecutiveDenials(scope string) int {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.consecutiveDenials[scope]
}

func (pm *PermissionManager) ShouldConfirmPermissions(scope string) bool {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.consecutiveDenials[scope] >= pm.maxConsecutiveDenials
}

func (pm *PermissionManager) AddRule(scope string, rule Rule) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	if scope == "" {
		pm.rules = append(pm.rules, rule)
		return
	}
	pm.scopedRules[scope] = append(pm.scopedRules[scope], rule)
}

func (pm *PermissionManager) ClearScope(scope string) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	delete(pm.scopedRules, scope)
	delete(pm.consecutiveDenials, scope)
}

func (pm *PermissionManager) GetRules() []Rule {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	rules := make([]Rule, len(pm.rules))
	copy(rules, pm.rules)
	return rules
}

func rulePattern(rule Rule) string {
	if rule.Tool == "bash" && rule.Content != "" {
		return "Bash(" + rule.Content + ")"
	}
	if rule.Path != "" {
		return rule.Tool + "(" + rule.Path + ")"
	}
	return rule.Tool
}

// PersistAlwaysAllow atomically appends one least-privilege rule to permission.json.
func PersistAlwaysAllow(path string, rule Rule) error {
	persistMu.Lock()
	defer persistMu.Unlock()

	var file permissionFile
	data, err := os.ReadFile(path)
	if err == nil {
		if err := json.Unmarshal(data, &file); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}

	pattern := rulePattern(rule)
	for _, existing := range file.Permissions.Allow {
		if existing == pattern {
			return nil
		}
	}
	file.Permissions.Allow = append(file.Permissions.Allow, pattern)
	out, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".permission-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0644); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(out); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}
