// Package userconfig 管理用户的运行配置与渠道绑定。
package userconfig

// AgentSettings 是用户可编辑的 Agent 配置，不包含任何密钥。
type AgentSettings struct {
	Personality    string
	DefaultModelID string
	Timezone       string
}

// ProviderCredential 是一个模型供应商的密钥。APIKey 为 nil 时保留已有密钥。
type ProviderCredential struct {
	ProviderID string
	APIKey     *string
}

// ProviderStatus 是可安全返回浏览器的供应商配置状态。
type ProviderStatus struct {
	ProviderID string
	HasAPIKey  bool
}

// ChannelBinding 将一个渠道账号映射到唯一的 Clerk 用户。
type ChannelBinding struct {
	Channel        string
	ExternalUserID string
	ClerkUserID    string
}

// MCPHeader 保存 MCP 请求 Header 的真实值，只能留在服务端。
type MCPHeader struct {
	Name  string
	Value string
}

// MCPHeaderInput 是可编辑的 MCP Header。Value 为 nil 时保留已有值。
type MCPHeaderInput struct {
	Name  string
	Value *string
}

// MCPServer 是一个用户的完整 MCP 配置，包含仅供服务端使用的 Header 值。
type MCPServer struct {
	ID        string
	Name      string
	URL       string
	Transport string
	Enabled   bool
	Headers   []MCPHeader
}

// MCPServerInput 是创建或更新 MCP 服务时可编辑的字段。
type MCPServerInput struct {
	Name      string
	URL       string
	Transport string
	Enabled   bool
	Headers   []MCPHeaderInput
}

// SettingsRequest 是保存用户设置的 HTTP 输入。UserID 由 BFF 注入。
type SettingsRequest struct {
	UserID         string                    `json:"userId"`
	Personality    string                    `json:"personality"`
	DefaultModelID string                    `json:"defaultModelId"`
	Timezone       string                    `json:"timezone"`
	Providers      []ProviderCredentialInput `json:"providers"`
}

// ProviderCredentialInput 是浏览器提交的供应商密钥。空值不会覆盖已存密钥。
type ProviderCredentialInput struct {
	ProviderID string  `json:"providerId"`
	APIKey     *string `json:"apiKey"`
}

// SettingsResponse 是读取或保存用户设置的 HTTP 输出，永不包含密钥。
type SettingsResponse struct {
	Personality    string                    `json:"personality"`
	DefaultModelID string                    `json:"defaultModelId"`
	Timezone       string                    `json:"timezone"`
	Providers      []ProviderCredentialState `json:"providers"`
}

// ProviderCredentialState 表示一个供应商是否已经保存密钥。
type ProviderCredentialState struct {
	ProviderID string `json:"providerId"`
	HasAPIKey  bool   `json:"hasApiKey"`
}

// MCPServerRequest 是 MCP 创建或更新的 HTTP 输入。UserID 由 BFF 注入。
type MCPServerRequest struct {
	UserID    string           `json:"userId"`
	Name      string           `json:"name"`
	URL       string           `json:"url"`
	Transport string           `json:"transport"`
	Enabled   bool             `json:"enabled"`
	Headers   []MCPHeaderInput `json:"headers"`
}

// MCPServerResponse 是浏览器可见的 MCP 服务，不包含 Header 值。
type MCPServerResponse struct {
	ID        string           `json:"id"`
	Name      string           `json:"name"`
	URL       string           `json:"url"`
	Transport string           `json:"transport"`
	Enabled   bool             `json:"enabled"`
	Headers   []MCPHeaderState `json:"headers"`
}

// MCPHeaderState 只表示 Header 是否已保存值。
type MCPHeaderState struct {
	Name     string `json:"name"`
	HasValue bool   `json:"hasValue"`
}

// MCPServerListResponse 保持空列表为 []。
type MCPServerListResponse struct {
	Servers []MCPServerResponse `json:"servers"`
}
