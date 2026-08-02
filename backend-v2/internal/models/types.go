// Package models 管理 EDITH 支持的模型目录与浏览器投影。
package models

import "trpc.group/trpc-go/trpc-agent-go/model"

const (
	DeepSeekProviderID    = "deepseek"
	MiniMaxProviderID     = "minimax"
	StepFunProviderID     = "stepfun"
	StepFunPlanProviderID = "stepfun_plan"
	XiaomiProviderID      = "xiaomi"

	DeepSeekV4FlashID  = "deepseek.v4.flash"
	DeepSeekV4ProID    = "deepseek.v4.pro"
	MiniMaxM3ID        = "minimax.m3"
	Step37FlashID      = "stepfun.step-3.7-flash"
	Step35FlashID      = "stepfun.step-3.5-flash"
	StepPlan37FlashID  = "stepfun_plan.step-3.7-flash"
	StepPlan35FlashID  = "stepfun_plan.step-3.5-flash"
	XiaomiMimoV25ProID = "xiaomi.mimo-v2.5-pro"
	XiaomiMimoV25ID    = "xiaomi.mimo-v2.5"
	DefaultModelID     = MiniMaxM3ID
)

// ProviderInfo 是用户需要配置 API Key 的供应商。
type ProviderInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// ReasoningOption 是一个模型专属的推理选项。
type ReasoningOption struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Capabilities 描述一个模型支持的能力。
type Capabilities struct {
	Vision bool `json:"vision"`
}

// Info 是可安全发送给浏览器的模型描述。
type Info struct {
	ID               string            `json:"id"`
	ProviderID       string            `json:"providerId"`
	Name             string            `json:"name"`
	ReasoningOptions []ReasoningOption `json:"reasoningOptions"`
	Capabilities     Capabilities      `json:"capabilities"`
}

// Definition 将浏览器目录项和 Runner 使用的模型适配器绑定。
type Definition struct {
	Info
	// ContextWindow 是该模型可承载的最大上下文 Token 数。
	// 它只供后端摘要阈值计算使用，不暴露给浏览器。
	ContextWindow                   int `json:"-"`
	Model                           model.Model
	DoesNotReportCachedPromptTokens bool
}

// CatalogResponse 是模型目录 HTTP 输出。
type CatalogResponse struct {
	Providers []ProviderInfo `json:"providers"`
	Models    []Info         `json:"models"`
}

// AvailableResponse 是已配置凭据的模型列表 HTTP 输出。
type AvailableResponse struct {
	Models []Info `json:"models"`
}
