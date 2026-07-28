package models

import "testing"

func TestRegisteredModels(t *testing.T) {
	tests := []struct {
		name      string
		modelName string
	}{
		{name: DeepSeekV4FlashID, modelName: "deepseek-v4-flash"},
		{name: MiniMaxM3ID, modelName: "MiniMax-M3"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Registered[tt.name]
			if m == nil {
				t.Fatal("model is not registered")
			}
			if got := m.Info().Name; got != tt.modelName {
				t.Fatalf("Info().Name = %q, want %q", got, tt.modelName)
			}
		})
	}
}
