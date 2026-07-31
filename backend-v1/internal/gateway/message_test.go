package gateway

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGatewayStreamOnlyAcceptsPost(t *testing.T) {
	server := &Server{}
	mux := http.NewServeMux()
	server.Register(mux)

	request := httptest.NewRequest(http.MethodGet, "/internal/gateway/messages:stream", nil)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusMethodNotAllowed)
	}
}

func TestGatewayStreamRejectsInvalidJSONBeforeUsingDependencies(t *testing.T) {
	server := &Server{}
	mux := http.NewServeMux()
	server.Register(mux)

	request := httptest.NewRequest(
		http.MethodPost,
		"/internal/gateway/messages:stream",
		strings.NewReader(`{"unexpected":true}`),
	)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
}
