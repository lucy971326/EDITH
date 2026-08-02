package models

import "testing"

func TestModelDefinitionsHaveContextWindows(t *testing.T) {
	expected := map[string]int{
		DeepSeekV4FlashID:  contextWindow1M,
		DeepSeekV4ProID:    contextWindow1M,
		MiniMaxM3ID:        contextWindow1M,
		Step37FlashID:      contextWindow256K,
		Step35FlashID:      contextWindow256K,
		StepPlan37FlashID:  contextWindow256K,
		StepPlan35FlashID:  contextWindow256K,
		XiaomiMimoV25ProID: contextWindow1M,
		XiaomiMimoV25ID:    contextWindow1M,
	}

	for _, definition := range modelDefinitions() {
		want, ok := expected[definition.ID]
		if !ok {
			t.Fatalf("model %q has no expected context window", definition.ID)
		}
		if definition.ContextWindow != want {
			t.Errorf("model %q ContextWindow = %d, want %d", definition.ID, definition.ContextWindow, want)
		}
		if got := definition.Model.Info().ContextWindow; got != want {
			t.Errorf("model %q adapter context window = %d, want %d", definition.ID, got, want)
		}
	}
}
