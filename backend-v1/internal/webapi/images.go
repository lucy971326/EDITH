package webapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"edith/backend-v1/internal/images"
)

func (s Server) createImageUpload(w http.ResponseWriter, r *http.Request) {
	request, err := decodeCreateImageUploadRequest(w, r)
	if err != nil {
		return
	}
	image, uploadURL, err := s.Images.CreateUpload(r.Context(), request.UserID, images.UploadInput{
		SessionID: request.SessionID,
		MimeType:  request.MimeType,
		SizeBytes: request.SizeBytes,
	})
	if err != nil {
		http.Error(w, "create image upload: "+err.Error(), http.StatusBadRequest)
		return
	}
	writeJSONStatus(w, http.StatusCreated, CreateImageUploadResponse{
		Image:     chatImage(image),
		UploadURL: uploadURL,
	})
}

func (s Server) completeImageUpload(w http.ResponseWriter, r *http.Request) {
	request, err := decodeCompleteImageUploadRequest(w, r)
	if err != nil {
		return
	}
	imageID := strings.TrimSpace(r.PathValue("imageID"))
	if imageID == "" {
		http.Error(w, "imageID is required", http.StatusBadRequest)
		return
	}
	image, err := s.Images.CompleteUpload(r.Context(), request.UserID, imageID)
	if err != nil {
		http.Error(w, "complete image upload: "+err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, CompleteImageUploadResponse{Image: chatImage(image)})
}

func (s Server) openImage(w http.ResponseWriter, r *http.Request) {
	userID := strings.TrimSpace(r.URL.Query().Get("userId"))
	imageID := strings.TrimSpace(r.PathValue("imageID"))
	if userID == "" || imageID == "" {
		http.Error(w, "userId and imageID are required", http.StatusBadRequest)
		return
	}
	url, err := s.Images.OpenForUser(r.Context(), userID, imageID)
	if err != nil {
		http.Error(w, "open image: "+err.Error(), http.StatusNotFound)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	http.Redirect(w, r, url, http.StatusFound)
}

func decodeCreateImageUploadRequest(w http.ResponseWriter, r *http.Request) (CreateImageUploadRequest, error) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	defer r.Body.Close()
	var request CreateImageUploadRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		http.Error(w, "invalid image upload request", http.StatusBadRequest)
		return CreateImageUploadRequest{}, err
	}
	request.UserID = strings.TrimSpace(request.UserID)
	request.SessionID = strings.TrimSpace(request.SessionID)
	request.MimeType = strings.TrimSpace(request.MimeType)
	if request.UserID == "" || request.SessionID == "" || request.MimeType == "" || request.SizeBytes <= 0 {
		http.Error(w, "userId, sessionId, mimeType, and sizeBytes are required", http.StatusBadRequest)
		return CreateImageUploadRequest{}, errors.New("missing image upload field")
	}
	return request, nil
}

func decodeCompleteImageUploadRequest(w http.ResponseWriter, r *http.Request) (CompleteImageUploadRequest, error) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	defer r.Body.Close()
	var request CompleteImageUploadRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		http.Error(w, "invalid image completion request", http.StatusBadRequest)
		return CompleteImageUploadRequest{}, err
	}
	request.UserID = strings.TrimSpace(request.UserID)
	if request.UserID == "" {
		http.Error(w, "userId is required", http.StatusBadRequest)
		return CompleteImageUploadRequest{}, errors.New("missing userId")
	}
	return request, nil
}

func chatImage(image images.Image) ChatImage {
	return ChatImage{ID: image.ID, MimeType: image.MimeType}
}
