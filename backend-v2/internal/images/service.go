package images

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

const (
	maxImageBytes = 10 << 20
	pendingStatus = "pending"
	readyStatus   = "ready"
)

type service struct {
	store *store
	cos   cosClient
}

// CreateUpload 预留图片并返回一次 COS 上传地址。
func (s *service) CreateUpload(ctx context.Context, userID string, input UploadInput) (Image, string, error) {
	userID = strings.TrimSpace(userID)
	input = normalizeUploadInput(input)
	if userID == "" || input.SessionID == "" {
		return Image{}, "", errors.New("userID and sessionID are required")
	}
	if !supportedMimeType(input.MimeType) {
		return Image{}, "", fmt.Errorf("unsupported image mime type %q", input.MimeType)
	}
	if input.SizeBytes <= 0 || input.SizeBytes > maxImageBytes {
		return Image{}, "", fmt.Errorf("image size must be between 1 and %d bytes", maxImageBytes)
	}

	image := Image{ID: "img_" + uuid.NewString(), MimeType: input.MimeType}
	record := imageRecord{Image: image, userID: userID, sessionID: input.SessionID, objectKey: "chat-images/" + image.ID, sizeBytes: input.SizeBytes, status: pendingStatus}
	if err := s.store.insert(ctx, record); err != nil {
		return Image{}, "", err
	}
	uploadURL, err := s.cos.signPut(ctx, record.objectKey)
	if err != nil {
		_ = s.store.delete(ctx, image.ID)
		return Image{}, "", err
	}
	return image, uploadURL, nil
}

// CompleteUpload 校验 COS 中的对象后，让图片可用于对话。
func (s *service) CompleteUpload(ctx context.Context, userID, imageID string) (Image, error) {
	record, err := s.store.forUser(ctx, userID, imageID, false)
	if err != nil {
		return Image{}, err
	}
	if record.status == readyStatus {
		return record.Image, nil
	}
	object, err := s.cos.head(ctx, record.objectKey)
	if err != nil {
		return Image{}, fmt.Errorf("verify uploaded image: %w", err)
	}
	if object.sizeBytes != record.sizeBytes || (object.mimeType != "" && object.mimeType != record.MimeType) {
		return Image{}, s.discardUpload(ctx, record, errors.New("uploaded image does not match reservation"))
	}
	if err := s.store.markReady(ctx, record.ID); err != nil {
		return Image{}, err
	}
	return record.Image, nil
}

// OpenForUser 返回用户自己图片的一次性读取地址。
func (s *service) OpenForUser(ctx context.Context, userID, imageID string) (string, error) {
	record, err := s.store.forUser(ctx, userID, imageID, true)
	if err != nil {
		return "", err
	}
	return s.cos.signGet(ctx, record.objectKey)
}

func (s *service) openForSession(ctx context.Context, userID, sessionID, imageID string) (string, error) {
	record, err := s.store.forSession(ctx, userID, sessionID, imageID)
	if err != nil {
		return "", err
	}
	return s.cos.signGet(ctx, record.objectKey)
}

func (s *service) discardUpload(ctx context.Context, record imageRecord, reason error) error {
	if err := s.cos.delete(ctx, record.objectKey); err != nil {
		return fmt.Errorf("%w; delete invalid image object: %v", reason, err)
	}
	if err := s.store.delete(ctx, record.ID); err != nil {
		return fmt.Errorf("%w; delete invalid image reservation: %v", reason, err)
	}
	return reason
}

func normalizeConfig(config Config) Config {
	config.Bucket = strings.TrimSpace(config.Bucket)
	config.Region = strings.TrimSpace(config.Region)
	config.SecretID = strings.TrimSpace(config.SecretID)
	config.SecretKey = strings.TrimSpace(config.SecretKey)
	return config
}

func validateConfig(config Config) error {
	if config.Bucket == "" || config.Region == "" || config.SecretID == "" || config.SecretKey == "" {
		return errors.New("COS bucket, region, secretID, and secretKey are required")
	}
	return nil
}

func normalizeUploadInput(input UploadInput) UploadInput {
	input.SessionID = strings.TrimSpace(input.SessionID)
	input.MimeType = strings.ToLower(strings.TrimSpace(input.MimeType))
	return input
}

func supportedMimeType(mimeType string) bool {
	return mimeType == "image/jpeg" || mimeType == "image/png" || mimeType == "image/webp"
}

// MaxImageBytes 返回浏览器可上传图片的最大字节数。
func MaxImageBytes() int64 { return maxImageBytes }
