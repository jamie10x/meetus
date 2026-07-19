package event

import (
	"context"
	"testing"
	"time"
)

// TestFirePublished_InvokesHookAsync guards the auto-announce wiring: once
// SetOnPublished is set, firePublished must invoke it (in the background,
// since callers run it after writing the HTTP response) with the published
// event.
func TestFirePublished_InvokesHookAsync(t *testing.T) {
	h := &Handler{}
	got := make(chan *Event, 1)
	h.SetOnPublished(func(ctx context.Context, e *Event) { got <- e })

	want := &Event{ID: 42}
	h.firePublished(want)

	select {
	case e := <-got:
		if e != want {
			t.Fatalf("hook called with %+v, want %+v", e, want)
		}
	case <-time.After(time.Second):
		t.Fatal("onPublished hook was not invoked")
	}
}

// TestFirePublished_NoHookIsNoop ensures unpublish/cancel (which pass a nil
// hook) and servers with no announcer configured don't panic.
func TestFirePublished_NoHookIsNoop(t *testing.T) {
	h := &Handler{}
	h.firePublished(&Event{ID: 1})
}
