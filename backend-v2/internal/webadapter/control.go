package webadapter

import (
	"net/http"

	"edith/backend-v2/internal/gateway"
	"edith/backend-v2/internal/httpx"
)

// RunStatus 返回 ManagedRunner 中仍属于当前用户的活跃任务。
func (a *Adapter) RunStatus(writer http.ResponseWriter, request *http.Request) {
	status, runError := a.gateway.Status(request.URL.Query().Get("userId"), request.PathValue("requestID"))
	if runError != nil {
		writeGatewayError(writer, runError)
		return
	}
	httpx.WriteJSON(writer, http.StatusOK, status)
}

// CancelRun 请求 ManagedRunner 停止一个属于当前用户的任务。
func (a *Adapter) CancelRun(writer http.ResponseWriter, request *http.Request) {
	if runError := a.gateway.Cancel(request.URL.Query().Get("userId"), request.PathValue("requestID")); runError != nil {
		writeGatewayError(writer, runError)
		return
	}
	writer.WriteHeader(http.StatusNoContent)
}

func writeGatewayError(writer http.ResponseWriter, runError *gateway.Error) {
	status := http.StatusInternalServerError
	switch runError.Type {
	case "invalid_request":
		status = http.StatusBadRequest
	case "session_busy", "request_conflict":
		status = http.StatusConflict
	case "identity_not_bound":
		status = http.StatusForbidden
	case "not_found":
		status = http.StatusNotFound
	}
	httpx.WriteError(writer, status, runError.Type, runError.Message)
}
