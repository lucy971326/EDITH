// Package httpx 提供各 HTTP 模块共同使用的 JSON 读写规则。
package httpx

import (
	"encoding/json"
	"net/http"
)

// Error 是所有 HTTP 模块共用的错误响应。
type Error struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// ReadJSON 从请求体读取一个 JSON 对象，并拒绝未知字段。
func ReadJSON(request *http.Request, target any) error {
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	return decoder.Decode(target)
}

// WriteJSON 写入指定状态码和 JSON 响应。
func WriteJSON(writer http.ResponseWriter, status int, value any) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
}

// WriteError 写入统一的 JSON 错误响应。
func WriteError(writer http.ResponseWriter, status int, errorType, message string) {
	WriteJSON(writer, status, Error{Type: errorType, Message: message})
}
