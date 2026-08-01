package models

import (
	"net/http"
	"strings"

	"edith/backend-v2/internal/httpx"
	"edith/backend-v2/internal/userconfig"
)

// HTTP 是 models 对 Web BFF 公开的 HTTP 能力。
type HTTP struct {
	catalog   *Catalog
	providers *userconfig.Providers
}

func (h *HTTP) listCatalog(writer http.ResponseWriter, request *http.Request) {
	httpx.WriteJSON(writer, http.StatusOK, CatalogResponse{
		Providers: h.catalog.Providers(),
		Models:    h.catalog.Models(),
	})
}

func (h *HTTP) listAvailable(writer http.ResponseWriter, request *http.Request) {
	userID := strings.TrimSpace(request.URL.Query().Get("userId"))
	if userID == "" {
		httpx.WriteError(writer, http.StatusBadRequest, "invalid_request", "userId is required")
		return
	}
	if h.providers == nil {
		httpx.WriteError(writer, http.StatusServiceUnavailable, "providers_unavailable", "provider configuration is unavailable")
		return
	}
	statuses, err := h.providers.ListStatuses(request.Context(), userID)
	if err != nil {
		httpx.WriteError(writer, http.StatusInternalServerError, "provider_load_failed", "load provider settings failed")
		return
	}
	hasKey := make(map[string]bool, len(statuses))
	for _, status := range statuses {
		hasKey[status.ProviderID] = status.HasAPIKey
	}
	available := []Info{}
	for _, model := range h.catalog.Models() {
		if hasKey[model.ProviderID] {
			available = append(available, model)
		}
	}
	httpx.WriteJSON(writer, http.StatusOK, AvailableResponse{Models: available})
}
