package authn

import (
	"testing"
	"time"
)

func TestTokenManager_RoundTrip(t *testing.T) {
	tm := NewTokenManager("secret", 15*time.Minute)
	token, err := tm.IssueAccess(7, time.Now())
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	userID, err := tm.ParseAccess(token)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if userID != 7 {
		t.Errorf("userID = %d, want 7", userID)
	}
}

func TestTokenManager_Expired(t *testing.T) {
	tm := NewTokenManager("secret", 15*time.Minute)
	token, err := tm.IssueAccess(7, time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	if _, err := tm.ParseAccess(token); err == nil {
		t.Fatal("expected error for expired token")
	}
}

func TestTokenManager_WrongSecret(t *testing.T) {
	token, err := NewTokenManager("secret-a", 15*time.Minute).IssueAccess(7, time.Now())
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	if _, err := NewTokenManager("secret-b", 15*time.Minute).ParseAccess(token); err == nil {
		t.Fatal("expected error for wrong secret")
	}
}

func TestTokenManager_Garbage(t *testing.T) {
	tm := NewTokenManager("secret", 15*time.Minute)
	if _, err := tm.ParseAccess("not-a-jwt"); err == nil {
		t.Fatal("expected error for malformed token")
	}
}
