package skills

import (
	"strings"
	"testing"
)

func TestParseFrontmatterSimple(t *testing.T) {
	text := `---
name: foo
description: bar
---

body here`
	meta, body := ParseFrontmatter(text)
	if meta["name"] != "foo" || meta["description"] != "bar" {
		t.Fatalf("meta = %v", meta)
	}
	if strings.TrimSpace(body) != "body here" {
		t.Fatalf("body = %q", body)
	}
}

func TestParseFrontmatterFoldedBlock(t *testing.T) {
	// 折叠块 `>-`：多行用空格拼接
	text := `---
name: learn-it
description: >-
  Use when 用户学完知识想回顾
  把对话知识总结为选择题
license: MIT
---

body`
	meta, _ := ParseFrontmatter(text)
	want := "Use when 用户学完知识想回顾 把对话知识总结为选择题"
	if meta["description"] != want {
		t.Fatalf("description = %q, want %q", meta["description"], want)
	}
	if meta["license"] != "MIT" {
		t.Fatalf("license = %q, want MIT", meta["license"])
	}
}

func TestParseFrontmatterLiteralBlock(t *testing.T) {
	text := `---
name: x
usage: |-
  line1
  line2
---

body`
	meta, _ := ParseFrontmatter(text)
	if meta["usage"] != "line1\nline2" {
		t.Fatalf("usage = %q, want %q", meta["usage"], "line1\nline2")
	}
}

func TestParseFrontmatterNestedMappingSkipped(t *testing.T) {
	text := `---
name: x
description: d
metadata:
  author: A
  version: 1.0.0
---

body`
	meta, _ := ParseFrontmatter(text)
	if meta["author"] != "" {
		t.Fatalf("nested metadata should be skipped, got author=%q", meta["author"])
	}
	if meta["name"] != "x" || meta["description"] != "d" {
		t.Fatalf("meta = %v", meta)
	}
}

func TestParseFrontmatterNoFrontmatter(t *testing.T) {
	text := "plain body without frontmatter"
	meta, body := ParseFrontmatter(text)
	if meta != nil {
		t.Fatalf("meta = %v, want nil", meta)
	}
	if body != text {
		t.Fatalf("body = %q, want original", body)
	}
}
