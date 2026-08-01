package userconfig

import (
	"errors"
	"net/http"
	"strings"

	"edith/backend-v2/internal/httpx"
)

// listMCPServers 返回当前用户配置的 MCP 服务。
func (h *HTTP) listMCPServers(writer http.ResponseWriter, request *http.Request) {
	userID := strings.TrimSpace(request.URL.Query().Get("userId"))
	if userID == "" {
		httpx.WriteError(writer, http.StatusBadRequest, "invalid_request", "userId is required")
		return
	}
	servers, err := h.mcp.List(request.Context(), userID)
	if err != nil {
		httpx.WriteError(writer, http.StatusInternalServerError, "mcp_list_failed", "list MCP servers failed")
		return
	}
	httpx.WriteJSON(writer, http.StatusOK, MCPServerListResponse{Servers: mcpResponses(servers)})
}

// createMCPServer 校验输入并创建一条 MCP 服务配置。
func (h *HTTP) createMCPServer(writer http.ResponseWriter, request *http.Request) {
	input, ok := readMCPServerRequest(writer, request)
	if !ok {
		return
	}
	server, err := h.mcp.Create(request.Context(), input.UserID, mcpInput(input))
	if err != nil {
		httpMCPError(writer, err)
		return
	}
	httpx.WriteJSON(writer, http.StatusCreated, mcpResponse(server))
}

// updateMCPServer 更新属于当前用户的一条 MCP 服务配置。
func (h *HTTP) updateMCPServer(writer http.ResponseWriter, request *http.Request) {
	input, ok := readMCPServerRequest(writer, request)
	if !ok {
		return
	}
	serverID := strings.TrimSpace(request.PathValue("serverID"))
	if serverID == "" {
		httpx.WriteError(writer, http.StatusBadRequest, "invalid_request", "serverID is required")
		return
	}
	server, err := h.mcp.Update(request.Context(), input.UserID, serverID, mcpInput(input))
	if err != nil {
		httpMCPError(writer, err)
		return
	}
	httpx.WriteJSON(writer, http.StatusOK, mcpResponse(server))
}

// deleteMCPServer 删除属于当前用户的一条 MCP 服务配置。
func (h *HTTP) deleteMCPServer(writer http.ResponseWriter, request *http.Request) {
	userID := strings.TrimSpace(request.URL.Query().Get("userId"))
	serverID := strings.TrimSpace(request.PathValue("serverID"))
	if userID == "" || serverID == "" {
		httpx.WriteError(writer, http.StatusBadRequest, "invalid_request", "userId and serverID are required")
		return
	}
	if err := h.mcp.Delete(request.Context(), userID, serverID); err != nil {
		httpMCPError(writer, err)
		return
	}
	writer.WriteHeader(http.StatusNoContent)
}

func readMCPServerRequest(writer http.ResponseWriter, request *http.Request) (MCPServerRequest, bool) {
	request.Body = http.MaxBytesReader(writer, request.Body, 1<<20)
	defer request.Body.Close()

	var input MCPServerRequest
	if err := httpx.ReadJSON(request, &input); err != nil {
		httpx.WriteError(writer, http.StatusBadRequest, "invalid_request", "invalid MCP server request")
		return MCPServerRequest{}, false
	}
	input.UserID = strings.TrimSpace(input.UserID)
	if input.UserID == "" {
		httpx.WriteError(writer, http.StatusBadRequest, "invalid_request", "userId is required")
		return MCPServerRequest{}, false
	}
	return input, true
}

func mcpInput(request MCPServerRequest) MCPServerInput {
	return MCPServerInput{
		Name:      request.Name,
		URL:       request.URL,
		Transport: request.Transport,
		Enabled:   request.Enabled,
		Headers:   request.Headers,
	}
}

func mcpResponses(servers []MCPServer) []MCPServerResponse {
	responses := make([]MCPServerResponse, 0, len(servers))
	for _, server := range servers {
		responses = append(responses, mcpResponse(server))
	}
	return responses
}

func mcpResponse(server MCPServer) MCPServerResponse {
	headers := make([]MCPHeaderState, 0, len(server.Headers))
	for _, header := range server.Headers {
		headers = append(headers, MCPHeaderState{
			Name:     header.Name,
			HasValue: header.Value != "",
		})
	}
	return MCPServerResponse{
		ID:        server.ID,
		Name:      server.Name,
		URL:       server.URL,
		Transport: server.Transport,
		Enabled:   server.Enabled,
		Headers:   headers,
	}
}

func httpMCPError(writer http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrInvalidMCPServer):
		httpx.WriteError(writer, http.StatusBadRequest, "invalid_mcp_server", err.Error())
	case errors.Is(err, ErrMCPServerNotFound):
		httpx.WriteError(writer, http.StatusNotFound, "mcp_server_not_found", "MCP server not found")
	default:
		httpx.WriteError(writer, http.StatusInternalServerError, "mcp_operation_failed", "MCP server operation failed")
	}
}
