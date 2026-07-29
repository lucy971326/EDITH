// Package images owns EDITH's private chat-image metadata and COS access.
package images

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
	"trpc.group/trpc-go/trpc-agent-go/model"
)

const (
	maxImageBytes = 10 << 20
	pendingStatus = "pending"
	readyStatus   = "ready"
)

// Config contains the private COS credentials used only by Go.
type Config struct {
	Bucket    string
	Region    string
	SecretID  string
	SecretKey string
}

// Service uses EDITH's image metadata database and owns its COS client.
type Service struct {
	db     *sql.DB
	cos    cosStore
	config Config
}

// Image is EDITH's durable image reference. It deliberately excludes the COS
// object key and any presigned URL.
type Image struct {
	ID       string
	MimeType string
}

type uploadInput struct {
	SessionID string
	MimeType  string
	SizeBytes int64
}

type imageRecord struct {
	Image
	UserID    string
	SessionID string
	ObjectKey string
	SizeBytes int64
	Status    string
}

// Open validates COS configuration and creates image tables on the
// caller-owned SQLite connection.
func Open(db *sql.DB, config Config) (*Service, error) {
	if db == nil {
		return nil, errors.New("image database is required")
	}
	config = normalizedConfig(config)
	if err := checkConfig(config); err != nil {
		return nil, err
	}

	client, err := newCOSStore(config)
	if err != nil {
		return nil, err
	}
	service := &Service{db: db, cos: client, config: config}
	if err := service.createTable(context.Background()); err != nil {
		return nil, err
	}
	return service, nil
}

// CreateUpload reserves one image before the browser uploads its bytes
// directly to COS.
func (s *Service) CreateUpload(ctx context.Context, userID string, input uploadInput) (Image, string, error) {
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
	record := imageRecord{
		Image:     image,
		UserID:    userID,
		SessionID: input.SessionID,
		ObjectKey: "chat-images/" + image.ID,
		SizeBytes: input.SizeBytes,
		Status:    pendingStatus,
	}
	if err := s.insert(ctx, record); err != nil {
		return Image{}, "", err
	}

	uploadURL, err := s.cos.signPut(ctx, record.ObjectKey)
	if err != nil {
		_ = s.delete(ctx, image.ID)
		return Image{}, "", err
	}
	return image, uploadURL, nil
}

// CompleteUpload verifies that the browser actually uploaded the reserved
// object, then makes the image eligible for a chat message.
func (s *Service) CompleteUpload(ctx context.Context, userID, imageID string) (Image, error) {
	record, err := s.loadForUser(ctx, userID, imageID)
	if err != nil {
		return Image{}, err
	}
	if record.Status == readyStatus {
		return record.Image, nil
	}

	object, err := s.cos.head(ctx, record.ObjectKey)
	if err != nil {
		return Image{}, fmt.Errorf("verify uploaded image: %w", err)
	}
	if object.SizeBytes != record.SizeBytes {
		return Image{}, s.discardUpload(ctx, record, errors.New("uploaded image size does not match reservation"))
	}
	if object.MimeType != "" && object.MimeType != record.MimeType {
		return Image{}, s.discardUpload(ctx, record, errors.New("uploaded image type does not match reservation"))
	}
	if err := s.markReady(ctx, record.ID); err != nil {
		return Image{}, err
	}
	return record.Image, nil
}

// discardUpload removes an invalid browser upload and its reservation. A
// presigned URL cannot impose every upload rule itself, so this is the final
// storage boundary after COS object verification.
func (s *Service) discardUpload(ctx context.Context, record imageRecord, reason error) error {
	if err := s.cos.delete(ctx, record.ObjectKey); err != nil {
		return fmt.Errorf("%w; discard invalid image object: %v", reason, err)
	}
	if err := s.delete(ctx, record.ID); err != nil {
		return fmt.Errorf("%w; delete invalid image reservation: %v", reason, err)
	}
	return reason
}

// OpenForUser signs a fresh read URL for a browser-owned image.
func (s *Service) OpenForUser(ctx context.Context, userID, imageID string) (string, error) {
	record, err := s.loadReadyForUser(ctx, userID, imageID)
	if err != nil {
		return "", err
	}
	return s.cos.signGet(ctx, record.ObjectKey)
}

// OpenForSession signs a fresh read URL only when the image belongs to this
// exact conversation. It is used while rebuilding model history.
func (s *Service) OpenForSession(ctx context.Context, userID, sessionID, imageID string) (string, error) {
	record, err := s.loadReadyForSession(ctx, userID, sessionID, imageID)
	if err != nil {
		return "", err
	}
	return s.cos.signGet(ctx, record.ObjectKey)
}

// AddMessageImages signs the selected images for one immediate model call and
// records their runtime URLs in ctx so Session persistence can replace them
// with durable EDITH image references.
func (s *Service) AddMessageImages(
	ctx context.Context,
	userID string,
	sessionID string,
	imageIDs []string,
	message *model.Message,
) (context.Context, error) {
	if len(imageIDs) == 0 {
		return ctx, nil
	}
	if message == nil {
		return nil, errors.New("message is required")
	}

	urls := make(map[string]string, len(imageIDs))
	seen := make(map[string]struct{}, len(imageIDs))
	for _, imageID := range imageIDs {
		imageID = strings.TrimSpace(imageID)
		if imageID == "" {
			return nil, errors.New("imageId is required")
		}
		if _, exists := seen[imageID]; exists {
			return nil, fmt.Errorf("image %q was supplied more than once", imageID)
		}
		seen[imageID] = struct{}{}

		record, err := s.loadReadyForSession(ctx, userID, sessionID, imageID)
		if err != nil {
			return nil, err
		}
		url, err := s.cos.signGet(ctx, record.ObjectKey)
		if err != nil {
			return nil, err
		}
		// Detail is an optional OpenAI-style extension, not a universal image
		// field. Leave it empty so each provider uses its own default quality.
		message.AddImageURL(url, "")
		urls[url] = imageID
	}
	return withMessageImageURLs(ctx, urls), nil
}

func normalizedConfig(config Config) Config {
	config.Bucket = strings.TrimSpace(config.Bucket)
	config.Region = strings.TrimSpace(config.Region)
	config.SecretID = strings.TrimSpace(config.SecretID)
	config.SecretKey = strings.TrimSpace(config.SecretKey)
	return config
}

func checkConfig(config Config) error {
	if config.Bucket == "" || config.Region == "" || config.SecretID == "" || config.SecretKey == "" {
		return errors.New("EDITH_COS_BUCKET, EDITH_COS_REGION, EDITH_COS_SECRET_ID, and EDITH_COS_SECRET_KEY are required")
	}
	return nil
}

func normalizeUploadInput(input uploadInput) uploadInput {
	input.SessionID = strings.TrimSpace(input.SessionID)
	input.MimeType = strings.ToLower(strings.TrimSpace(input.MimeType))
	return input
}

func supportedMimeType(mimeType string) bool {
	switch mimeType {
	case "image/jpeg", "image/png", "image/webp":
		return true
	default:
		return false
	}
}

// MaxImageBytes is exposed to HTTP validation and tests without duplicating a
// browser-facing limit.
func MaxImageBytes() int64 { return maxImageBytes }

// UploadInput is the HTTP-independent input used to reserve an image.
type UploadInput = uploadInput

// SignedURLLifetime is intentionally short because COS URLs are bearer URLs.
const SignedURLLifetime = 5 * time.Minute
