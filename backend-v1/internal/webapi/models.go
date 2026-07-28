package webapi

import (
	"encoding/json"
	"net/http"

	"edith/backend-v1/internal/models"
)

func (s Server) listModels(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	response := ModelCatalogResponse{Providers: models.Providers, Models: models.Catalog}
	encoder := json.NewEncoder(w)
	err := encoder.Encode(response)
	if err != nil {
		http.Error(w, "encode model catalog: "+err.Error(), http.StatusInternalServerError)
	}
}

func (s Server) listAvailableModels(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("userId")
	settings, providers, err := s.Users.LoadSettings(r.Context(), userID)
	_ = settings
	if err != nil {
		http.Error(w, "load available models: "+err.Error(), http.StatusInternalServerError)
		return
	}
	hasKey := map[string]bool{}
	for _, provider := range providers {
		hasKey[provider.ProviderID] = provider.HasAPIKey
	}
	// Keep the JSON contract stable: an empty catalog is [], never null.
	available := []models.Info{}
	for _, definition := range models.Catalog {
		if hasKey[definition.ProviderID] {
			available = append(available, definition)
		}
	}
	writeJSON(w, struct {
		Models []models.Info `json:"models"`
	}{Models: available})
}
