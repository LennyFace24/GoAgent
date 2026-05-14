package skilltool

import (
	"context"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

type LoadSkillInput struct {
	Name string `json:"name" description:"要调用的技能名称" required:"true"`
}

type SkillLoader interface {
	LoadFullText(name string) string
}

func NewSkillTool(loader SkillLoader) (tool.InvokableTool, error) {
	return utils.InferTool(
		"skill",
		"加载指定技能的完整内容。调用后技能规则将在后续对话中生效。",
		func(ctx context.Context, input LoadSkillInput) (string, error) {
			return loader.LoadFullText(input.Name), nil
		},
	)
}
