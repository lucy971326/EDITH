package userconfig

// ProviderCredential is one user's credential for one model provider. APIKey
// is nil when the browser intentionally leaves an already stored key unchanged.
type ProviderCredential struct {
	ProviderID string
	APIKey     *string
}

// ProviderStatus is safe to return to the browser: it reveals configuration
// state but never the credential itself.
type ProviderStatus struct {
	ProviderID string
	HasAPIKey  bool
}

// Settings is the editable user-level configuration. It contains data only;
// MCP, Skills, and Sandbox behavior belong to their own packages.
type Settings struct {
	Personality    string
	DefaultModelID string
	Providers      []ProviderCredential
}

// ChannelBinding 将一个渠道账号绑定到唯一的 Clerk 用户。
// 外部渠道永远不是 EDITH 的第二套用户系统，只是 Clerk 用户的消息入口。
type ChannelBinding struct {
	Channel        string
	ExternalUserID string
	UserID         string
}
