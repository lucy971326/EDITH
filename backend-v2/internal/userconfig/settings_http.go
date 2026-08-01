package userconfig

import (
	"net/http"
	"strings"

	"edith/backend-v2/internal/httpx"
)

// HTTP 是 userconfig 对 Web BFF 公开的 HTTP 能力。
type HTTP struct {
	settings       *Settings
	providers      *Providers
	mcp            *MCP
	defaultModelID string
}

// getSettings 按 BFF 注入的 userId 返回用户设置。
func (h *HTTP) getSettings(writer http.ResponseWriter, request *http.Request) {
	userID := strings.TrimSpace(request.URL.Query().Get("userId"))
	if userID == "" {
		httpx.WriteError(writer, http.StatusBadRequest, "invalid_request", "userId is required")
		return
	}
	h.writeSettings(writer, request, userID)
}

// saveSettings 保存 Agent 设置和模型供应商凭据。
func (h *HTTP) saveSettings(writer http.ResponseWriter, request *http.Request) {
	request.Body = http.MaxBytesReader(writer, request.Body, 1<<20)
	defer request.Body.Close()

	var input SettingsRequest
	if err := httpx.ReadJSON(request, &input); err != nil {
		httpx.WriteError(writer, http.StatusBadRequest, "invalid_request", "invalid user settings request")
		return
	}
	input.UserID = strings.TrimSpace(input.UserID)
	input.Personality = strings.TrimSpace(input.Personality)
	input.DefaultModelID = strings.TrimSpace(input.DefaultModelID)
	input.Timezone = strings.TrimSpace(input.Timezone)
	if input.UserID == "" || input.DefaultModelID == "" {
		httpx.WriteError(writer, http.StatusBadRequest, "invalid_request", "userId and defaultModelId are required")
		return
	}

	credentials := make([]ProviderCredential, 0, len(input.Providers))
	for _, provider := range input.Providers {
		provider.ProviderID = strings.TrimSpace(provider.ProviderID)
		if provider.ProviderID == "" {
			httpx.WriteError(writer, http.StatusBadRequest, "invalid_request", "providerId is required")
			return
		}
		if provider.APIKey != nil {
			apiKey := strings.TrimSpace(*provider.APIKey)
			if apiKey == "" {
				provider.APIKey = nil
			} else {
				provider.APIKey = &apiKey
			}
		}
		credentials = append(credentials, ProviderCredential{
			ProviderID: provider.ProviderID,
			APIKey:     provider.APIKey,
		})
	}

	settings := AgentSettings{
		Personality:    input.Personality,
		DefaultModelID: input.DefaultModelID,
		Timezone:       input.Timezone,
	}
	if err := h.settings.Save(request.Context(), input.UserID, settings); err != nil {
		httpx.WriteError(writer, http.StatusInternalServerError, "settings_save_failed", "save user settings failed")
		return
	}
	if err := h.providers.Save(request.Context(), input.UserID, credentials); err != nil {
		httpx.WriteError(writer, http.StatusInternalServerError, "provider_save_failed", "save provider settings failed")
		return
	}
	h.writeSettings(writer, request, input.UserID)
}

func (h *HTTP) writeSettings(writer http.ResponseWriter, request *http.Request, userID string) {
	settings, err := h.settings.Load(request.Context(), userID)
	if err != nil {
		httpx.WriteError(writer, http.StatusInternalServerError, "settings_load_failed", "load user settings failed")
		return
	}
	statuses, err := h.providers.ListStatuses(request.Context(), userID)
	if err != nil {
		httpx.WriteError(writer, http.StatusInternalServerError, "provider_load_failed", "load provider settings failed")
		return
	}
	providers := make([]ProviderCredentialState, 0, len(statuses))
	for _, status := range statuses {
		providers = append(providers, ProviderCredentialState{
			ProviderID: status.ProviderID,
			HasAPIKey:  status.HasAPIKey,
		})
	}
	defaultModelID := settings.DefaultModelID
	if defaultModelID == "" {
		defaultModelID = h.defaultModelID
	}
	httpx.WriteJSON(writer, http.StatusOK, SettingsResponse{
		Personality:    settings.Personality,
		DefaultModelID: defaultModelID,
		Timezone:       settings.Timezone,
		Providers:      providers,
	})
}
