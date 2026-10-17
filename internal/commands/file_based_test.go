package commands

import "testing"

func TestRenderTemplatePositionalArgs(t *testing.T) {
	body := "分析 $1 模块，重点是 $2"
	got := RenderTemplate(body, []string{"auth", "权限"})
	want := "分析 auth 模块，重点是 权限"
	if got != want {
		t.Fatalf("RenderTemplate = %q, want %q", got, want)
	}
}

func TestRenderTemplateAggregateArgs(t *testing.T) {
	body := "任务: $ARGUMENTS"
	got := RenderTemplate(body, []string{"a", "b", "c"})
	want := "任务: a b c"
	if got != want {
		t.Fatalf("RenderTemplate = %q, want %q", got, want)
	}
}

func TestRenderTemplateNoPlaceholderAppendsArgs(t *testing.T) {
	// 模板未用占位符时，参数追加到末尾（omp inline-argument fallback）
	got := RenderTemplate("展示当前状态", []string{"详细", "模式"})
	want := "展示当前状态\n\n详细 模式"
	if got != want {
		t.Fatalf("RenderTemplate = %q, want %q", got, want)
	}
}

func TestRenderTemplateNoArgsKeepsBody(t *testing.T) {
	got := RenderTemplate("展示当前状态", nil)
	if got != "展示当前状态" {
		t.Fatalf("RenderTemplate = %q, want unchanged body", got)
	}
}

func TestRenderTemplateEmptyArgLeavesPlaceholderEmpty(t *testing.T) {
	// $2 未提供时替换为空字符串，不追加（因为 $1 占位符已消费参数）
	got := RenderTemplate("A=$1 B=$2", []string{"x"})
	if got != "A=x B=" {
		t.Fatalf("RenderTemplate = %q, want %q", got, "A=x B=")
	}
}
