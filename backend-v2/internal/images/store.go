package images

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// store 是图片模块私有的 SQLite 访问器。
type store struct{ db *sql.DB }

type imageRecord struct {
	Image
	userID, sessionID, objectKey string
	sizeBytes                    int64
	status                       string
}

func (s *store) createSchema(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS chat_images (
		image_id TEXT PRIMARY KEY, user_id TEXT NOT NULL, session_id TEXT NOT NULL,
		object_key TEXT NOT NULL UNIQUE, mime_type TEXT NOT NULL, size_bytes INTEGER NOT NULL,
		status TEXT NOT NULL, created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`)
	if err != nil {
		return fmt.Errorf("create chat images table: %w", err)
	}
	return nil
}

func (s *store) insert(ctx context.Context, record imageRecord) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO chat_images
		(image_id, user_id, session_id, object_key, mime_type, size_bytes, status)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		record.ID, record.userID, record.sessionID, record.objectKey, record.MimeType, record.sizeBytes, record.status)
	if err != nil {
		return fmt.Errorf("save image %q: %w", record.ID, err)
	}
	return nil
}

func (s *store) delete(ctx context.Context, imageID string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM chat_images WHERE image_id = ?", imageID)
	return err
}

func (s *store) markReady(ctx context.Context, imageID string) error {
	_, err := s.db.ExecContext(ctx, "UPDATE chat_images SET status = ? WHERE image_id = ?", readyStatus, imageID)
	return err
}

func (s *store) forUser(ctx context.Context, userID, imageID string, readyOnly bool) (imageRecord, error) {
	query := `SELECT image_id, user_id, session_id, object_key, mime_type, size_bytes, status
		FROM chat_images WHERE image_id = ? AND user_id = ?`
	args := []any{strings.TrimSpace(imageID), strings.TrimSpace(userID)}
	if readyOnly {
		query += " AND status = ?"
		args = append(args, readyStatus)
	}
	return s.read(ctx, query, args...)
}

func (s *store) forSession(ctx context.Context, userID, sessionID, imageID string) (imageRecord, error) {
	return s.read(ctx, `SELECT image_id, user_id, session_id, object_key, mime_type, size_bytes, status
		FROM chat_images WHERE image_id = ? AND user_id = ? AND session_id = ? AND status = ?`,
		strings.TrimSpace(imageID), strings.TrimSpace(userID), strings.TrimSpace(sessionID), readyStatus)
}

func (s *store) read(ctx context.Context, query string, args ...any) (imageRecord, error) {
	var record imageRecord
	err := s.db.QueryRowContext(ctx, query, args...).Scan(&record.ID, &record.userID, &record.sessionID, &record.objectKey, &record.MimeType, &record.sizeBytes, &record.status)
	if errors.Is(err, sql.ErrNoRows) {
		return imageRecord{}, errors.New("image not found")
	}
	if err != nil {
		return imageRecord{}, fmt.Errorf("read image: %w", err)
	}
	return record, nil
}
