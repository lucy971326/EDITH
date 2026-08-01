package images

import (
	"net/http"
	"strings"

	"edith/backend-v2/internal/httpx"
)

// HTTP 是图片模块对 Web BFF 提供的 HTTP 能力。
type HTTP struct{ service *service }

// CreateUpload 接收上传预留请求，返回图片身份和 COS 上传地址。
func (h *HTTP) CreateUpload(writer http.ResponseWriter, request *http.Request) {
	request.Body = http.MaxBytesReader(writer, request.Body, 1<<20)
	defer request.Body.Close()
	var input CreateUploadRequest
	if err := httpx.ReadJSON(request, &input); err != nil {
		httpx.WriteError(writer, http.StatusBadRequest, "invalid_request", "invalid image upload request")
		return
	}
	input.UserID, input.SessionID, input.MimeType = strings.TrimSpace(input.UserID), strings.TrimSpace(input.SessionID), strings.TrimSpace(input.MimeType)
	if input.UserID == "" || input.SessionID == "" || input.MimeType == "" || input.SizeBytes <= 0 {
		httpx.WriteError(writer, http.StatusBadRequest, "invalid_request", "userId, sessionId, mimeType, and sizeBytes are required")
		return
	}
	image, uploadURL, err := h.service.CreateUpload(request.Context(), input.UserID, UploadInput{SessionID: input.SessionID, MimeType: input.MimeType, SizeBytes: input.SizeBytes})
	if err != nil {
		httpx.WriteError(writer, http.StatusBadRequest, "image_upload_rejected", err.Error())
		return
	}
	httpx.WriteJSON(writer, http.StatusCreated, CreateUploadResponse{Image: image, UploadURL: uploadURL})
}

// CompleteUpload 确认指定图片已上传且可安全用于对话。
func (h *HTTP) CompleteUpload(writer http.ResponseWriter, request *http.Request) {
	var input CompleteUploadRequest
	if err := httpx.ReadJSON(request, &input); err != nil {
		httpx.WriteError(writer, http.StatusBadRequest, "invalid_request", "invalid image completion request")
		return
	}
	imageID := strings.TrimSpace(request.PathValue("imageID"))
	input.UserID = strings.TrimSpace(input.UserID)
	if input.UserID == "" || imageID == "" {
		httpx.WriteError(writer, http.StatusBadRequest, "invalid_request", "userId and imageID are required")
		return
	}
	image, err := h.service.CompleteUpload(request.Context(), input.UserID, imageID)
	if err != nil {
		httpx.WriteError(writer, http.StatusBadRequest, "image_upload_rejected", err.Error())
		return
	}
	httpx.WriteJSON(writer, http.StatusOK, CompleteUploadResponse{Image: image})
}

// OpenImage 重定向到当前用户图片的短期读取地址。
func (h *HTTP) OpenImage(writer http.ResponseWriter, request *http.Request) {
	userID := strings.TrimSpace(request.URL.Query().Get("userId"))
	imageID := strings.TrimSpace(request.PathValue("imageID"))
	if userID == "" || imageID == "" {
		httpx.WriteError(writer, http.StatusBadRequest, "invalid_request", "userId and imageID are required")
		return
	}
	url, err := h.service.OpenForUser(request.Context(), userID, imageID)
	if err != nil {
		httpx.WriteError(writer, http.StatusNotFound, "image_not_found", "image not found")
		return
	}
	writer.Header().Set("Cache-Control", "no-store")
	http.Redirect(writer, request, url, http.StatusFound)
}
