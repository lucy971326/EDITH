package images

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

func (s *Service) createTable(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS chat_images (
		image_id TEXT PRIMARY KEY,
		user_id TEXT NOT NULL,
		session_id TEXT NOT NULL,
		object_key TEXT NOT NULL UNIQUE,
		mime_type TEXT NOT NULL,
		size_bytes INTEGER NOT NULL,
		status TEXT NOT NULL,
		created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`)
	if err != nil {
		return fmt.Errorf("create chat images table: %w", err)
	}
	return nil
}

func (s *Service) insert(ctx context.Context, record imageRecord) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO chat_images (
		image_id, user_id, session_id, object_key, mime_type, size_bytes, status
	) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		record.ID, record.UserID, record.SessionID, record.ObjectKey,
		record.MimeType, record.SizeBytes, record.Status,
	)
	if err != nil {
		return fmt.Errorf("save image %q: %w", record.ID, err)
	}
	return nil
}

func (s *Service) delete(ctx context.Context, imageID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM chat_images WHERE image_id = ?`, imageID)
	return err
}

func (s *Service) markReady(ctx context.Context, imageID string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE chat_images SET status = ? WHERE image_id = ?`, readyStatus, imageID)
	if err != nil {
		return fmt.Errorf("mark image %q ready: %w", imageID, err)
	}
	return nil
}

func (s *Service) loadForUser(ctx context.Context, userID, imageID string) (imageRecord, error) {
	return s.load(ctx, `SELECT image_id, user_id, session_id, object_key, mime_type, size_bytes, status
		FROM chat_images WHERE image_id = ? AND user_id = ?`, imageID, strings.TrimSpace(userID))
}

func (s *Service) loadReadyForUser(ctx context.Context, userID, imageID string) (imageRecord, error) {
	return s.load(ctx, `SELECT image_id, user_id, session_id, object_key, mime_type, size_bytes, status
		FROM chat_images WHERE image_id = ? AND user_id = ? AND status = ?`, imageID, strings.TrimSpace(userID), readyStatus)
}

func (s *Service) loadReadyForSession(ctx context.Context, userID, sessionID, imageID string) (imageRecord, error) {
	return s.load(ctx, `SELECT image_id, user_id, session_id, object_key, mime_type, size_bytes, status
		FROM chat_images WHERE image_id = ? AND user_id = ? AND session_id = ? AND status = ?`,
		imageID, strings.TrimSpace(userID), strings.TrimSpace(sessionID), readyStatus)
}

func (s *Service) load(ctx context.Context, query string, args ...any) (imageRecord, error) {
	var record imageRecord
	err := s.db.QueryRowContext(ctx, query, args...).Scan(
		&record.ID,
		&record.UserID,
		&record.SessionID,
		&record.ObjectKey,
		&record.MimeType,
		&record.SizeBytes,
		&record.Status,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return imageRecord{}, errors.New("image not found")
	}
	if err != nil {
		return imageRecord{}, fmt.Errorf("load image: %w", err)
	}
	return record, nil
}
