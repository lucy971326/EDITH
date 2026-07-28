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
	Personality string
	Providers   []ProviderCredential
}
