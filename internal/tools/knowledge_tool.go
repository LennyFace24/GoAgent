package tools

import (
	"context"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

// FileSearcher 是对文件搜索能力的抽象，用于打破 service <-> tools 的循环依赖
type FileSearcher interface {
	Search(ctx context.Context, query string, topK int) ([]string, error)
}

// 定义输入结构体
type KnowledgeSearchInput struct {
    Query string `json:"query" description:"搜索查询" required:"true"`
    TopK  int    `json:"top_k" description:"返回结果数量" required:"false"`
}

// 定义工具函数
func NewKnowledgeSearchTool(fileSearcher FileSearcher) (tool.InvokableTool, error) {
	return utils.InferTool(
		"knowledge_search",
		"从知识库中搜索相关文档，用于回答用户问题",
		func(ctx context.Context, input KnowledgeSearchInput) (string, error) {
			if input.TopK == 0{
				input.TopK = 5
			}
			results,err := fileSearcher.Search(ctx, input.Query, input.TopK)
			if err != nil {
				return "", err
			}
			return strings.Join(results, "\n---\n"), nil
		},
	)
}