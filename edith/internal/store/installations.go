package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

type Installation struct {
	InstallationID int64
	ClerkUserID    string
}

// SaveInstallation 按 installation_id 主键保存。
func (s *Store) SaveInstallation(ctx context.Context, ins Installation) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO github_installations (installation_id, clerk_user_id)
		 VALUES (?, ?)
		 ON CONFLICT(installation_id) DO UPDATE SET clerk_user_id = excluded.clerk_user_id`,
		ins.InstallationID, ins.ClerkUserID,
	)
	if err != nil {
		return fmt.Errorf("store: save installation %d: %w", ins.InstallationID, err)
	}
	return nil
}

// FindInstallationByID 查不到返回 ErrNotFound。
func (s *Store) FindInstallationByID(ctx context.Context, installationID int64) (Installation, error) {
	var ins Installation
	err := s.db.QueryRowContext(ctx,
		`SELECT installation_id, clerk_user_id FROM github_installations WHERE installation_id = ?`,
		installationID,
	).Scan(&ins.InstallationID, &ins.ClerkUserID)
	if errors.Is(err, sql.ErrNoRows) {
		return Installation{}, ErrNotFound
	}
	if err != nil {
		return Installation{}, fmt.Errorf("store: find installation %d: %w", installationID, err)
	}
	return ins, nil
}