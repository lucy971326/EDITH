package webapi

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"edith/backend-v1/internal/models"
	"edith/backend-v1/internal/userconfig"
)

func TestModelsReturnsCatalog(t *testing.T) {
	var server Server
	mux := http.NewServeMux()
	server.Register(mux)

	request := httptest.NewRequest(http.MethodGet, "/internal/models", nil)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}

	var responseBody ModelCatalogResponse
	if err := json.NewDecoder(response.Body).Decode(&responseBody); err != nil {
		t.Fatalf("decode catalog: %v", err)
	}
	if len(responseBody.Models) != len(models.Catalog) {
		t.Fatalf("catalog length = %d, want %d", len(responseBody.Models), len(models.Catalog))
	}
}

func TestUserSettingsEmptyProvidersIsJSONArray(t *testing.T) {
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "edith.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	store, err := userconfig.Open(db)
	if err != nil {
		t.Fatal(err)
	}

	server := Server{Users: store}
	mux := http.NewServeMux()
	server.Register(mux)
	request := httptest.NewRequest(http.MethodGet, "/internal/user-settings?userId=alice", nil)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if strings.Contains(response.Body.String(), `"providers":null`) {
		t.Fatalf("response contains null providers: %s", response.Body.String())
	}
}
