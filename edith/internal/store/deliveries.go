package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/mattn/go-sqlite3"
)

// TryInsertDelivery 尝试插入；主键冲突返回 (false, nil)。
func (s *Store) TryInsertDelivery(ctx context.Context, deliveryID string) (bool, error) {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO webhook_deliveries (delivery_id) VALUES (?)`,
		deliveryID,
	)
	if err == nil {
		return true, nil
	}
	var sqliteErr sqlite3.Error
	if errors.As(err, &sqliteErr) && sqliteErr.Code == sqlite3.ErrConstraint {
		return false, nil
	}
	return false, fmt.Errorf("store: insert delivery %s: %w", deliveryID, err)
}