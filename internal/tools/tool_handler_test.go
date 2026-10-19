package tools

import (
	"context"
	"testing"

	"github.com/LennyFace24/MiniAgent/internal/config"
	"github.com/LennyFace24/MiniAgent/internal/metrics"
	"github.com/LennyFace24/MiniAgent/internal/modelmgr"
)

type stubSearcher struct{}

func (stubSearcher) Search(context.Context, string, int) ([]string, error) { return nil, nil }

// 部分模型服务端要求 function.parameters 必须是 object；非 object 会在请求阶段直接 503。
func TestToolParametersAreObjects(t *testing.T) {
	if _, err := config.LoadConfig("../../config.yaml"); err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	t.Chdir(t.TempDir())

	handler, err := NewToolHandler(stubSearcher{}, metrics.NewCollector(), func() modelmgr.Model {
		return modelmgr.Model{}
	})
	if err != nil {
		t.Fatal(err)
	}

	for _, registered := range handler.Tools() {
		info, err := registered.Info(context.Background())
		if err != nil {
			t.Errorf("%s Info: %v", info.Name, err)
			continue
		}
		if info.ParamsOneOf == nil {
			continue
		}
		params, err := info.ParamsOneOf.ToJSONSchema()
		if err != nil {
			t.Errorf("%s schema: %v", info.Name, err)
			continue
		}
		if params != nil && params.Type != "object" {
			t.Errorf("%s parameters type = %q, want object", info.Name, params.Type)
		}
	}
}
