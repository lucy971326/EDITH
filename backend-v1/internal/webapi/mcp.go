package webapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"edith/backend-v1/internal/userconfig"
)

func (s Server) listMCPServers(w http.ResponseWriter, r *http.Request) {
	userID := strings.TrimSpace(r.URL.Query().Get("userId"))
	if userID == "" {
		http.Error(w, "userId is required", http.StatusBadRequest)
		return
	}
	servers, err := s.Users.ListMCPServers(r.Context(), userID)
	if err != nil {
		http.Error(w, "list MCP servers: "+err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, MCPServerListResponse{Servers: mcpServerResponses(servers)})
}

func (s Server) createMCPServer(w http.ResponseWriter, r *http.Request) {
	request, err := decodeMCPServerRequest(w, r)
	if err != nil {
		return
	}
	server, err := s.Users.CreateMCPServer(r.Context(), request.UserID, mcpServerInput(request))
	if err != nil {
		writeMCPServerError(w, err)
		return
	}
	writeJSONStatus(w, http.StatusCreated, mcpServerResponse(server))
}

func (s Server) updateMCPServer(w http.ResponseWriter, r *http.Request) {
	request, err := decodeMCPServerRequest(w, r)
	if err != nil {
		return
	}
	serverID := strings.TrimSpace(r.PathValue("serverID"))
	if serverID == "" {
		http.Error(w, "serverID is required", http.StatusBadRequest)
		return
	}
	server, err := s.Users.UpdateMCPServer(r.Context(), request.UserID, serverID, mcpServerInput(request))
	if err != nil {
		writeMCPServerError(w, err)
		return
	}
	writeJSON(w, mcpServerResponse(server))
}

func (s Server) deleteMCPServer(w http.ResponseWriter, r *http.Request) {
	userID := strings.TrimSpace(r.URL.Query().Get("userId"))
	serverID := strings.TrimSpace(r.PathValue("serverID"))
	if userID == "" || serverID == "" {
		http.Error(w, "userId and serverID are required", http.StatusBadRequest)
		return
	}
	if err := s.Users.DeleteMCPServer(r.Context(), userID, serverID); err != nil {
		writeMCPServerError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func decodeMCPServerRequest(w http.ResponseWriter, r *http.Request) (MCPServerRequest, error) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	defer r.Body.Close()
	var request MCPServerRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		http.Error(w, "invalid MCP server request", http.StatusBadRequest)
		return MCPServerRequest{}, err
	}
	request.UserID = strings.TrimSpace(request.UserID)
	request.Name = strings.TrimSpace(request.Name)
	request.URL = strings.TrimSpace(request.URL)
	request.Transport = strings.TrimSpace(request.Transport)
	if request.UserID == "" {
		http.Error(w, "userId is required", http.StatusBadRequest)
		return MCPServerRequest{}, errors.New("missing userId")
	}
	for index := range request.Headers {
		request.Headers[index].Name = strings.TrimSpace(request.Headers[index].Name)
		if request.Headers[index].Value == nil {
			continue
		}
		value := strings.TrimSpace(*request.Headers[index].Value)
		if value == "" {
			request.Headers[index].Value = nil
		} else {
			request.Headers[index].Value = &value
		}
	}
	return request, nil
}

func mcpServerInput(request MCPServerRequest) userconfig.MCPServerInput {
	input := userconfig.MCPServerInput{
		Name:      request.Name,
		URL:       request.URL,
		Transport: request.Transport,
		Enabled:   request.Enabled,
	}
	for _, header := range request.Headers {
		input.Headers = append(input.Headers, userconfig.MCPHeaderInput{Name: header.Name, Value: header.Value})
	}
	return input
}

func mcpServerResponses(servers []userconfig.MCPServer) []MCPServerResponse {
	responses := make([]MCPServerResponse, 0, len(servers))
	for _, server := range servers {
		responses = append(responses, mcpServerResponse(server))
	}
	return responses
}

func mcpServerResponse(server userconfig.MCPServer) MCPServerResponse {
	response := MCPServerResponse{
		ID:        server.ID,
		Name:      server.Name,
		URL:       server.URL,
		Transport: server.Transport,
		Enabled:   server.Enabled,
		Headers:   []MCPHeaderState{},
	}
	for _, header := range server.Headers {
		response.Headers = append(response.Headers, MCPHeaderState{Name: header.Name, HasValue: header.Value != ""})
	}
	return response
}

func writeMCPServerError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, userconfig.ErrInvalidMCPServer):
		http.Error(w, err.Error(), http.StatusBadRequest)
	case errors.Is(err, userconfig.ErrMCPServerNotFound):
		http.Error(w, "MCP server not found", http.StatusNotFound)
	default:
		http.Error(w, "MCP server operation: "+err.Error(), http.StatusInternalServerError)
	}
}
