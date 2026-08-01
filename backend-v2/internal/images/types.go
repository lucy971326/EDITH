package images

// Image 是浏览器和 Agent 可见的持久图片身份。
type Image struct {
	ID       string `json:"id"`
	MimeType string `json:"mimeType"`
}

// UploadInput 是预留浏览器上传图片所需的信息。
type UploadInput struct {
	SessionID string
	MimeType  string
	SizeBytes int64
}

// CreateUploadRequest 是 Next BFF 创建上传预留时传入的 HTTP 契约。
type CreateUploadRequest struct {
	UserID    string `json:"userId"`
	SessionID string `json:"sessionId"`
	MimeType  string `json:"mimeType"`
	SizeBytes int64  `json:"sizeBytes"`
}

// CreateUploadResponse 是浏览器直传 COS 所需的预留结果。
type CreateUploadResponse struct {
	Image     Image  `json:"image"`
	UploadURL string `json:"uploadUrl"`
}

// CompleteUploadRequest 是 Next BFF 确认上传完成时传入的 HTTP 契约。
type CompleteUploadRequest struct {
	UserID string `json:"userId"`
}

// CompleteUploadResponse 是确认成功后的图片身份。
type CompleteUploadResponse struct {
	Image Image `json:"image"`
}
