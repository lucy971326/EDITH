// Package models is EDITH's single source of truth for supported providers
// and models. Browser code only consumes its HTTP catalog; it never defines a
// model itself.
package models

import (
	"trpc.group/trpc-go/trpc-agent-go/model"
	"trpc.group/trpc-go/trpc-agent-go/model/openai"
)

const (
	DeepSeekProviderID = "deepseek"
	MiniMaxProviderID  = "minimax"

	DeepSeekV4FlashID = "deepseek.v4.flash"
	MiniMaxM3ID       = "minimax.m3"

	deepSeekBaseURL = "https://api.deepseek.com"
	miniMaxBaseURL  = "https://api.minimaxi.com/v1"
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

// Info is the safe, browser-facing description of a concrete selectable
// model. ID is EDITH's stable runtime key; Name is display-only.
type Info struct {
	ID               string            `json:"id"`
	ProviderID       string            `json:"providerId"`
	Name             string            `json:"name"`
	ReasoningOptions []ReasoningOption `json:"reasoningOptions"`
}

// Definition binds one catalog entry to the shared model adapter used by the
// Runner. It contains no user credentials.
type Definition struct {
	Info
	Model model.Model
}

var (
	// DeepSeekV4Flash is shared by every user. Credentials are loaded per run.
	DeepSeekV4Flash = openai.New("deepseek-v4-flash",
		openai.WithVariant(openai.VariantDeepSeek),
		openai.WithBaseURL(deepSeekBaseURL),
	)

	// MiniMaxM3 is shared by every user. Provider request configuration belongs
	// to this definition, never to a user's stored credential.
	MiniMaxM3 = openai.New("MiniMax-M3",
		openai.WithVariant(openai.VariantMiniMax),
		openai.WithBaseURL(miniMaxBaseURL),
		openai.WithExtraFields(map[string]any{
			"reasoning_split": true,
		}),
	)

	Providers = []ProviderInfo{
		{ID: DeepSeekProviderID, Name: "DeepSeek"},
		{ID: MiniMaxProviderID, Name: "MiniMax"},
	}

	// Definitions is the model catalog and adapter registry in one place. Add
	// later model variants here with a new stable ID; never add user API keys.
	Definitions = map[string]Definition{
		DeepSeekV4FlashID: {
			Info: Info{
				ID:               DeepSeekV4FlashID,
				ProviderID:       DeepSeekProviderID,
				Name:             "DeepSeek V4 Flash",
				ReasoningOptions: []ReasoningOption{},
			},
			Model: DeepSeekV4Flash,
		},
		MiniMaxM3ID: {
			Info: Info{
				ID:               MiniMaxM3ID,
				ProviderID:       MiniMaxProviderID,
				Name:             "MiniMax M3",
				ReasoningOptions: []ReasoningOption{},
			},
			Model: MiniMaxM3,
		},
	}

	// Catalog is ordered for UI display. Keep it aligned with Definitions.
	Catalog = []Info{
		Definitions[DeepSeekV4FlashID].Info,
		Definitions[MiniMaxM3ID].Info,
	}

	// Registered is the mapping consumed by agent.WithModelName.
	Registered = map[string]model.Model{
		DeepSeekV4FlashID: DeepSeekV4Flash,
		MiniMaxM3ID:       MiniMaxM3,
	}
)

// Lookup returns the definition for one stable EDITH model ID.
func Lookup(modelID string) (Definition, bool) {
	definition, ok := Definitions[modelID]
	return definition, ok
}
