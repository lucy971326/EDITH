package usage

// Run 是一次 Agent 运行的稳定身份。
type Run struct {
	RequestID string
	UserID    string
	SessionID string
	ModelID   string
}

// Tokens 是一次运行累计的供应商 Token 用量。
type Tokens struct {
	PromptTokens       int
	CachedPromptTokens *int
	CompletionTokens   int
	TotalTokens        int
}

// Summary 是一个会话的浏览器可展示用量汇总。
type Summary struct {
	TotalTokens          int      `json:"totalTokens"`
	CachedPromptTokens   *int     `json:"cachedPromptTokens"`
	UncachedPromptTokens *int     `json:"uncachedPromptTokens"`
	CompletionTokens     int      `json:"completionTokens"`
	CacheHitRate         *float64 `json:"cacheHitRate"`
}
