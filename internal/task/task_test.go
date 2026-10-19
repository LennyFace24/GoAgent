package task

import (
	"context"
	"strings"
	"testing"
)

func TestGetTaskSchemaIsObjectAndInvocationWorks(t *testing.T) {
	t.Chdir(t.TempDir())
	manager := NewTaskManager()

	create, err := manager.CreateTaskTool()
	if err != nil {
		t.Fatal(err)
	}
	created, err := create.InvokableRun(context.Background(), `{
		"subject":"schema test",
		"description":"ensure get_task accepts an object",
		"status":"pending",
		"block_by":[],
		"blocking":[],
		"owner":"tester"
	}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(created, "ID: 1") {
		t.Fatalf("unexpected create result: %q", created)
	}

	get, err := manager.GetTaskTool()
	if err != nil {
		t.Fatal(err)
	}
	info, err := get.Info(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	params, err := info.ParamsOneOf.ToJSONSchema()
	if err != nil {
		t.Fatal(err)
	}
	if params.Type != "object" {
		t.Fatalf("get_task parameters type = %q, want object", params.Type)
	}
	if _, ok := params.Properties.Get("id"); !ok {
		t.Fatalf("get_task parameters missing id: %+v", params.Properties)
	}

	detail, err := get.InvokableRun(context.Background(), `{"id":1}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(detail, "schema test") {
		t.Fatalf("get_task returned wrong task: %q", detail)
	}
}
