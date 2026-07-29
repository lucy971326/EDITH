package models

import "testing"

func TestRegisteredModels(t *testing.T) {
	tests := []struct {
		name      string
		modelName string
		vision    bool
	}{
		{name: DeepSeekV4FlashID, modelName: "deepseek-v4-flash", vision: false},
		{name: DeepSeekV4ProID, modelName: "deepseek-v4-pro", vision: false},
		{name: MiniMaxM3ID, modelName: "MiniMax-M3", vision: true},
		{name: Step37FlashID, modelName: "step-3.7-flash", vision: true},
		{name: Step35FlashID, modelName: "step-3.5-flash", vision: true},
		{name: StepPlan37FlashID, modelName: "step-3.7-flash", vision: true},
		{name: StepPlan35FlashID, modelName: "step-3.5-flash", vision: false},
		{name: XiaomiMimoV25ProID, modelName: "mimo-v2.5-pro", vision: false},
		{name: XiaomiMimoV25ID, modelName: "mimo-v2.5", vision: true},
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
			definition, ok := Lookup(tt.name)
			if !ok {
				t.Fatal("model definition is missing")
			}
			if got := definition.Capabilities.Vision; got != tt.vision {
				t.Fatalf("Capabilities.Vision = %v, want %v", got, tt.vision)
			}
		})
	}
}
