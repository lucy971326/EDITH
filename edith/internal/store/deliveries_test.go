package store

import (
	"context"
	"testing"
)

func TestTryInsertDelivery_FirstAndDuplicate(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	inserted, err := s.TryInsertDelivery(ctx, "del_001")
	if err != nil || !inserted {
		t.Fatalf("first insert: inserted=%v err=%v, want (true, nil)", inserted, err)
	}

	inserted, err = s.TryInsertDelivery(ctx, "del_001")
	if err != nil || inserted {
		t.Fatalf("duplicate insert: inserted=%v err=%v, want (false, nil)", inserted, err)
	}

	// 不同的 ID 仍然能插入
	inserted, err = s.TryInsertDelivery(ctx, "del_002")
	if err != nil || !inserted {
		t.Fatalf("second id insert: inserted=%v err=%v, want (true, nil)", inserted, err)
	}
}