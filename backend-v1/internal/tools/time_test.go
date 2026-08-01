package tools

import (
	"context"
	"encoding/json"
	"testing"
)

func TestGetCurrentTime(t *testing.T) {
	args, err := json.Marshal(getCurrentTimeInput{Timezone: "Asia/Shanghai"})
	if err != nil {
		t.Fatal(err)
	}

	result, err := GetCurrentTime.Call(context.Background(), args)
	if err != nil {
		t.Fatalf("Call() error = %v", err)
	}

	output, ok := result.(getCurrentTimeOutput)
	if !ok {
		t.Fatalf("Call() result type = %T, want getCurrentTimeOutput", result)
	}
	if output.Time == "" || output.Timezone != "Asia/Shanghai" {
		t.Fatalf("Call() output = %#v", output)
	}
}

func TestDefaultContainsGetCurrentTime(t *testing.T) {
	defaults := Default(nil, nil)
	for _, item := range defaults.Tools {
		if item.Declaration().Name == "get_current_time" {
			return
		}
	}
	t.Fatal("Default does not contain get_current_time")
}
