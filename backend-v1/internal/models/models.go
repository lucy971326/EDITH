// Package models is EDITH's single source of truth for supported providers
// and models. Browser code only consumes its HTTP catalog; it never defines a
// model itself.
package models

import (
	"trpc.group/trpc-go/trpc-agent-go/model"
	"trpc.group/trpc-go/trpc-agent-go/model/openai"
)

const (
	DeepSeekProviderID    = "deepseek"
	MiniMaxProviderID     = "minimax"
	StepFunProviderID     = "stepfun"
	StepFunPlanProviderID = "stepfun_plan"
	XiaomiProviderID      = "xiaomi"

	DeepSeekV4FlashID = "deepseek.v4.flash"
	DeepSeekV4ProID   = "deepseek.v4.pro"
	MiniMaxM3ID       = "minimax.m3"

	// Standard StepFun API models
	Step37FlashID = "stepfun.step-3.7-flash"
	Step35FlashID = "stepfun.step-3.5-flash"

	// StepFun Plan models (use step_plan baseURL)
	StepPlan37FlashID = "stepfun_plan.step-3.7-flash"
	StepPlan35FlashID = "stepfun_plan.step-3.5-flash"

	// Xiaomi MiMo models
	XiaomiMimoV25ProID = "xiaomi.mimo-v2.5-pro"
	XiaomiMimoV25ID    = "xiaomi.mimo-v2.5"

	DefaultModelID = MiniMaxM3ID

	deepSeekBaseURL = "https://api.deepseek.com"
	miniMaxBaseURL  = "https://api.minimaxi.com/v1"
	stepFunBaseURL  = "https://api.stepfun.com/v1"
	stepPlanBaseURL = "https://api.stepfun.com/step_plan/v1"
	xiaomiBaseURL   = "https://api.xiaomimimo.com/v1"
)

// ProviderInfo identifies the service where a user owns an API credential.
type ProviderInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// ReasoningOption is a model-scoped, opaque choice. Its ID is not a global
// enum: each model may expose a completely different set of options.
type ReasoningOption struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Capabilities describes what one concrete model can do. It is model-scoped:
// models from the same provider may expose different capabilities.
type Capabilities struct {
	Vision bool `json:"vision"`
}

// Info is the safe, browser-facing description of a concrete selectable
// model. ID is EDITH's stable runtime key; Name is display-only.
type Info struct {
	ID               string            `json:"id"`
	ProviderID       string            `json:"providerId"`
	Name             string            `json:"name"`
	ReasoningOptions []ReasoningOption `json:"reasoningOptions"`
	Capabilities     Capabilities      `json:"capabilities"`
}

// Definition binds one catalog entry to the shared model adapter used by the
// Runner. It contains no user credentials.
type Definition struct {
	Info
	Model model.Model
	// DoesNotReportCachedPromptTokens is an exceptional provider capability.
	// Cache metrics are enabled by default; set this only after confirming that
	// a provider omits cached prompt token data.
	DoesNotReportCachedPromptTokens bool
}

var (
	// Providers is the credential registry. A user configures one API key per
	// provider, regardless of how many concrete models that provider offers.
	Providers = []ProviderInfo{
		{ID: DeepSeekProviderID, Name: "DeepSeek"},
		{ID: MiniMaxProviderID, Name: "MiniMax"},
		{ID: StepFunProviderID, Name: "阶跃星辰"},
		{ID: StepFunPlanProviderID, Name: "阶跃星辰 (Step Plan)"},
		{ID: XiaomiProviderID, Name: "小米 MiMo"},
	}

	// Definitions is EDITH's ordered model registry and the only place a
	// concrete model is declared. It binds browser-facing metadata to its
	// shared adapter; user credentials are still supplied per run.
	Definitions = []Definition{
		{
			Info: Info{
				ID:               DeepSeekV4FlashID,
				ProviderID:       DeepSeekProviderID,
				Name:             "DeepSeek V4 Flash",
				ReasoningOptions: []ReasoningOption{},
				Capabilities:     Capabilities{Vision: false},
			},
			Model: openai.New("deepseek-v4-flash",
				openai.WithVariant(openai.VariantDeepSeek),
				openai.WithBaseURL(deepSeekBaseURL),
			),
		},
		{
			Info: Info{
				ID:               DeepSeekV4ProID,
				ProviderID:       DeepSeekProviderID,
				Name:             "DeepSeek V4 Pro",
				ReasoningOptions: []ReasoningOption{},
				Capabilities:     Capabilities{Vision: false},
			},
			Model: openai.New("deepseek-v4-pro",
				openai.WithVariant(openai.VariantDeepSeek),
				openai.WithBaseURL(deepSeekBaseURL),
			),
		},
		{
			Info: Info{
				ID:               MiniMaxM3ID,
				ProviderID:       MiniMaxProviderID,
				Name:             "MiniMax M3",
				ReasoningOptions: []ReasoningOption{},
				Capabilities:     Capabilities{Vision: true},
			},
			Model: openai.New("MiniMax-M3",
				openai.WithVariant(openai.VariantMiniMax),
				openai.WithBaseURL(miniMaxBaseURL),
				openai.WithExtraFields(map[string]any{
					"reasoning_split": true,
				}),
			),
		},
		// Standard StepFun API models (api.stepfun.com/v1)
		{
			Info: Info{
				ID:               Step37FlashID,
				ProviderID:       StepFunProviderID,
				Name:             "Step 3.7 Flash",
				ReasoningOptions: []ReasoningOption{},
				Capabilities:     Capabilities{Vision: true},
			},
			Model: openai.New("step-3.7-flash",
				openai.WithBaseURL(stepFunBaseURL),
			),
		},
		{
			Info: Info{
				ID:               Step35FlashID,
				ProviderID:       StepFunProviderID,
				Name:             "Step 3.5 Flash",
				ReasoningOptions: []ReasoningOption{},
				Capabilities:     Capabilities{Vision: true},
			},
			Model: openai.New("step-3.5-flash",
				openai.WithBaseURL(stepFunBaseURL),
			),
		},
		// StepFun Plan models (api.stepfun.com/step_plan/v1)
		{
			Info: Info{
				ID:               StepPlan37FlashID,
				ProviderID:       StepFunPlanProviderID,
				Name:             "Step 3.7 Flash (Step Plan)",
				ReasoningOptions: []ReasoningOption{},
				Capabilities:     Capabilities{Vision: true},
			},
			Model: openai.New("step-3.7-flash",
				openai.WithBaseURL(stepPlanBaseURL),
			),
		},
		{
			Info: Info{
				ID:               StepPlan35FlashID,
				ProviderID:       StepFunPlanProviderID,
				Name:             "Step 3.5 Flash (Step Plan)",
				ReasoningOptions: []ReasoningOption{},
				Capabilities:     Capabilities{Vision: false},
			},
			Model: openai.New("step-3.5-flash",
				openai.WithBaseURL(stepPlanBaseURL),
			),
		},
		// Xiaomi MiMo models (api.xiaomimimo.com/v1)
		{
			Info: Info{
				ID:               XiaomiMimoV25ProID,
				ProviderID:       XiaomiProviderID,
				Name:             "MiMo v2.5 Pro",
				ReasoningOptions: []ReasoningOption{},
				Capabilities:     Capabilities{Vision: false},
			},
			Model: openai.New("mimo-v2.5-pro",
				openai.WithBaseURL(xiaomiBaseURL),
			),
		},
		{
			Info: Info{
				ID:               XiaomiMimoV25ID,
				ProviderID:       XiaomiProviderID,
				Name:             "MiMo v2.5",
				ReasoningOptions: []ReasoningOption{},
				Capabilities:     Capabilities{Vision: true},
			},
			Model: openai.New("mimo-v2.5",
				openai.WithBaseURL(xiaomiBaseURL),
			),
		},
	}

	// Catalog is the browser-facing projection of Definitions.
	Catalog = catalogFrom(Definitions)

	// Registered is the Runner-facing projection of Definitions.
	Registered = registeredFrom(Definitions)
)

// Lookup returns the definition for one stable EDITH model ID.
func Lookup(modelID string) (Definition, bool) {
	for _, definition := range Definitions {
		if definition.ID == modelID {
			return definition, true
		}
	}
	return Definition{}, false
}

func catalogFrom(definitions []Definition) []Info {
	catalog := make([]Info, 0, len(definitions))
	for _, definition := range definitions {
		catalog = append(catalog, definition.Info)
	}
	return catalog
}

func registeredFrom(definitions []Definition) map[string]model.Model {
	registered := make(map[string]model.Model, len(definitions))
	for _, definition := range definitions {
		registered[definition.ID] = definition.Model
	}
	return registered
}
