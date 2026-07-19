package tgbot

import (
	"context"
	"testing"
	"time"

	"meetus.uz/backend/internal/config"
	"meetus.uz/backend/internal/platform/redisx"
)

// TestFeedbackCommentPending exercises the Redis-backed pending-comment
// marker used to turn the attendee's very next free-text message into a
// feedback comment: set it, confirm it's readable exactly once, and
// confirm a second pop finds nothing (GETDEL already consumed it).
func TestFeedbackCommentPending(t *testing.T) {
	ctx := context.Background()
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	rdb, err := redisx.NewClient(ctx, cfg.RedisAddr)
	if err != nil {
		t.Skipf("redis unavailable: %v", err)
	}
	t.Cleanup(func() { rdb.Close() })

	const telegramID = 900000401
	const eventID = 777
	t.Cleanup(func() { rdb.Del(ctx, feedbackAwaitKey(telegramID)) })

	b := &Bot{redis: rdb}

	if _, ok := b.popPendingFeedbackComment(ctx, telegramID); ok {
		t.Fatal("expected no pending comment before awaitFeedbackComment")
	}

	b.awaitFeedbackComment(ctx, telegramID, eventID)

	got, ok := b.popPendingFeedbackComment(ctx, telegramID)
	if !ok || got != eventID {
		t.Fatalf("popPendingFeedbackComment = (%d, %v), want (%d, true)", got, ok, eventID)
	}

	if _, ok := b.popPendingFeedbackComment(ctx, telegramID); ok {
		t.Fatal("expected pending comment to be consumed after first pop")
	}
}

// TestFeedbackCommentTTLIsBounded guards against accidentally removing the
// TTL — an unbounded key would mean a random later message from an inactive
// conversation gets silently swallowed as a "comment".
func TestFeedbackCommentTTLIsBounded(t *testing.T) {
	if feedbackCommentTTL <= 0 || feedbackCommentTTL > time.Hour {
		t.Fatalf("feedbackCommentTTL = %v, want a short bounded window", feedbackCommentTTL)
	}
}
