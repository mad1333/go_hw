package postgres

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestWaitForDatabase(t *testing.T) {
	calls := 0
	err := waitForDatabase(context.Background(), func(context.Context) error {
		calls++
		if calls < 3 {
			return errors.New("connection refused")
		}
		return nil
	})
	if err != nil || calls != 3 {
		t.Fatalf("calls=%d err=%v", calls, err)
	}
}
func TestWaitForDatabaseCancellation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	err := waitForDatabase(ctx, func(context.Context) error { return errors.New("connection refused") })
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal(err)
	}
}
