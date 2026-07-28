package webapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"edith/backend-v1/internal/models"
	"edith/backend-v1/internal/userconfig"
)

func (s Server) getUserSettings(w http.ResponseWriter, r *http.Request) {
	userID := strings.TrimSpace(r.URL.Query().Get("userId"))
	if userID == "" {
		http.Error(w, "userId is required", http.StatusBadRequest)
		return
	}
	settings, statuses, err := s.Users.LoadSettings(r.Context(), userID)
	if err != nil {
		http.Error(w, "load user settings: "+err.Error(), http.StatusInternalServerError)
		return
	}
	response := UserSettingsResponse{
		Personality: settings.Personality,
		Providers:   []ProviderCredentialState{},
	}
	for _, status := range statuses {
		response.Providers = append(response.Providers, ProviderCredentialState{ProviderID: status.ProviderID, HasAPIKey: status.HasAPIKey})
	}
	writeJSON(w, response)
}

func (s Server) saveUserSettings(w http.ResponseWriter, r *http.Request) {
	request, err := decodeUserSettingsRequest(w, r)
	if err != nil {
		return
	}
	settings := userconfig.Settings{Personality: request.Personality}
	for _, provider := range request.Providers {
		if !isKnownProvider(provider.ProviderID) {
			http.Error(w, "unsupported providerId", http.StatusBadRequest)
			return
		}
		settings.Providers = append(settings.Providers, userconfig.ProviderCredential{ProviderID: provider.ProviderID, APIKey: provider.APIKey})
	}
	if err := s.Users.SaveSettings(r.Context(), request.UserID, settings); err != nil {
		http.Error(w, "save user settings: "+err.Error(), http.StatusInternalServerError)
		return
	}
	loaded, statuses, err := s.Users.LoadSettings(r.Context(), request.UserID)
	if err != nil {
		http.Error(w, "load saved user settings: "+err.Error(), http.StatusInternalServerError)
		return
	}
	response := UserSettingsResponse{
		Personality: loaded.Personality,
		Providers:   []ProviderCredentialState{},
	}
	for _, status := range statuses {
		response.Providers = append(response.Providers, ProviderCredentialState{
			ProviderID: status.ProviderID,
			HasAPIKey:  status.HasAPIKey,
		})
	}
	writeJSON(w, response)
}

func decodeUserSettingsRequest(w http.ResponseWriter, r *http.Request) (UserSettingsRequest, error) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	defer r.Body.Close()
	var request UserSettingsRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		http.Error(w, "invalid user settings request", http.StatusBadRequest)
		return UserSettingsRequest{}, err
	}
	request.UserID = strings.TrimSpace(request.UserID)
	request.Personality = strings.TrimSpace(request.Personality)
	if request.UserID == "" {
		http.Error(w, "userId is required", http.StatusBadRequest)
		return UserSettingsRequest{}, errors.New("missing userId")
	}
	for index := range request.Providers {
		request.Providers[index].ProviderID = strings.TrimSpace(request.Providers[index].ProviderID)
		if request.Providers[index].APIKey != nil {
			key := strings.TrimSpace(*request.Providers[index].APIKey)
			if key == "" {
				request.Providers[index].APIKey = nil
			} else {
				request.Providers[index].APIKey = &key
			}
		}
	}
	return request, nil
}

func isKnownProvider(providerID string) bool {
	for _, provider := range models.Providers {
		if provider.ID == providerID {
			return true
		}
	}
	return false
}

func writeJSON(w http.ResponseWriter, value any) {
	writeJSONStatus(w, http.StatusOK, value)
}

func writeJSONStatus(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	encoder := json.NewEncoder(w)
	err := encoder.Encode(value)
	if err != nil {
		http.Error(w, "encode JSON response: "+err.Error(), http.StatusInternalServerError)
	}
}
