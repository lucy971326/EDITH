package models

import "trpc.group/trpc-go/trpc-agent-go/model/openai"

const (
	deepSeekBaseURL = "https://api.deepseek.com"
	miniMaxBaseURL  = "https://api.minimaxi.com/v1"
	stepFunBaseURL  = "https://api.stepfun.com/v1"
	stepPlanBaseURL = "https://api.stepfun.com/step_plan/v1"
	xiaomiBaseURL   = "https://api.xiaomimimo.com/v1"
)

func providerDefinitions() []ProviderInfo {
	return []ProviderInfo{
		{ID: DeepSeekProviderID, Name: "DeepSeek"},
		{ID: MiniMaxProviderID, Name: "MiniMax"},
		{ID: StepFunProviderID, Name: "阶跃星辰"},
		{ID: StepFunPlanProviderID, Name: "阶跃星辰 (Step Plan)"},
		{ID: XiaomiProviderID, Name: "小米 MiMo"},
	}
}

func modelDefinitions() []Definition {
	return []Definition{
		{
			Info: Info{
				ID: DeepSeekV4FlashID, ProviderID: DeepSeekProviderID, Name: "DeepSeek V4 Flash",
				ReasoningOptions: []ReasoningOption{}, Capabilities: Capabilities{Vision: false},
			},
			Model: openai.New("deepseek-v4-flash", openai.WithVariant(openai.VariantDeepSeek), openai.WithBaseURL(deepSeekBaseURL)),
		},
		{
			Info: Info{
				ID: DeepSeekV4ProID, ProviderID: DeepSeekProviderID, Name: "DeepSeek V4 Pro",
				ReasoningOptions: []ReasoningOption{}, Capabilities: Capabilities{Vision: false},
			},
			Model: openai.New("deepseek-v4-pro", openai.WithVariant(openai.VariantDeepSeek), openai.WithBaseURL(deepSeekBaseURL)),
		},
		{
			Info: Info{
				ID: MiniMaxM3ID, ProviderID: MiniMaxProviderID, Name: "MiniMax M3",
				ReasoningOptions: []ReasoningOption{}, Capabilities: Capabilities{Vision: true},
			},
			Model: openai.New("MiniMax-M3", openai.WithBaseURL(miniMaxBaseURL), openai.WithExtraFields(map[string]any{"reasoning_split": true})),
		},
		{
			Info: Info{
				ID: Step37FlashID, ProviderID: StepFunProviderID, Name: "Step 3.7 Flash",
				ReasoningOptions: []ReasoningOption{}, Capabilities: Capabilities{Vision: true},
			},
			Model: openai.New("step-3.7-flash", openai.WithBaseURL(stepFunBaseURL)),
		},
		{
			Info: Info{
				ID: Step35FlashID, ProviderID: StepFunProviderID, Name: "Step 3.5 Flash",
				ReasoningOptions: []ReasoningOption{}, Capabilities: Capabilities{Vision: true},
			},
			Model: openai.New("step-3.5-flash", openai.WithBaseURL(stepFunBaseURL)),
		},
		{
			Info: Info{
				ID: StepPlan37FlashID, ProviderID: StepFunPlanProviderID, Name: "Step 3.7 Flash (Step Plan)",
				ReasoningOptions: []ReasoningOption{}, Capabilities: Capabilities{Vision: true},
			},
			Model: openai.New("step-3.7-flash", openai.WithBaseURL(stepPlanBaseURL)),
		},
		{
			Info: Info{
				ID: StepPlan35FlashID, ProviderID: StepFunPlanProviderID, Name: "Step 3.5 Flash (Step Plan)",
				ReasoningOptions: []ReasoningOption{}, Capabilities: Capabilities{Vision: false},
			},
			Model: openai.New("step-3.5-flash", openai.WithBaseURL(stepPlanBaseURL)),
		},
		{
			Info: Info{
				ID: XiaomiMimoV25ProID, ProviderID: XiaomiProviderID, Name: "MiMo v2.5 Pro",
				ReasoningOptions: []ReasoningOption{}, Capabilities: Capabilities{Vision: false},
			},
			Model: openai.New("mimo-v2.5-pro", openai.WithBaseURL(xiaomiBaseURL)),
		},
		{
			Info: Info{
				ID: XiaomiMimoV25ID, ProviderID: XiaomiProviderID, Name: "MiMo v2.5",
				ReasoningOptions: []ReasoningOption{}, Capabilities: Capabilities{Vision: true},
			},
			Model: openai.New("mimo-v2.5", openai.WithBaseURL(xiaomiBaseURL)),
		},
	}
}
