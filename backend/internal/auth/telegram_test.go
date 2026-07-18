package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

const testBotToken = "123456:TEST-TOKEN"

// signFields replicates Telegram's signing so tests can produce valid payloads.
func signFields(t *testing.T, fields map[string]string, botToken string) string {
	t.Helper()
	keys := make([]string, 0, len(fields))
	for k := range fields {
		if k != "hash" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	lines := make([]string, 0, len(keys))
	for _, k := range keys {
		lines = append(lines, k+"="+fields[k])
	}
	secret := sha256.Sum256([]byte(botToken))
	mac := hmac.New(sha256.New, secret[:])
	mac.Write([]byte(strings.Join(lines, "\n")))
	return hex.EncodeToString(mac.Sum(nil))
}

func validFields(t *testing.T, now time.Time) map[string]string {
	t.Helper()
	fields := map[string]string{
		"id":         "42",
		"first_name": "Jamshid",
		"last_name":  "Boynazarov",
		"username":   "jamshid",
		"photo_url":  "https://t.me/i/userpic/42.jpg",
		"auth_date":  strconv.FormatInt(now.Unix(), 10),
	}
	fields["hash"] = signFields(t, fields, testBotToken)
	return fields
}

func TestVerifyTelegramLogin_Valid(t *testing.T) {
	now := time.Now()
	u, err := VerifyTelegramLogin(validFields(t, now), testBotToken, now)
	if err != nil {
		t.Fatalf("expected valid login, got error: %v", err)
	}
	if u.ID != 42 {
		t.Errorf("ID = %d, want 42", u.ID)
	}
	if got := u.DisplayName(); got != "Jamshid Boynazarov" {
		t.Errorf("DisplayName = %q", got)
	}
}

func TestVerifyTelegramLogin_TamperedField(t *testing.T) {
	now := time.Now()
	fields := validFields(t, now)
	fields["id"] = "999"
	if _, err := VerifyTelegramLogin(fields, testBotToken, now); err == nil {
		t.Fatal("expected error for tampered payload")
	}
}

func TestVerifyTelegramLogin_WrongBotToken(t *testing.T) {
	now := time.Now()
	if _, err := VerifyTelegramLogin(validFields(t, now), "other:token", now); err == nil {
		t.Fatal("expected error for wrong bot token")
	}
}

func TestVerifyTelegramLogin_MissingHash(t *testing.T) {
	now := time.Now()
	fields := validFields(t, now)
	delete(fields, "hash")
	if _, err := VerifyTelegramLogin(fields, testBotToken, now); err == nil {
		t.Fatal("expected error for missing hash")
	}
}

func TestVerifyTelegramLogin_ExpiredAuthDate(t *testing.T) {
	authTime := time.Now().Add(-25 * time.Hour)
	fields := validFields(t, authTime)
	if _, err := VerifyTelegramLogin(fields, testBotToken, time.Now()); err == nil {
		t.Fatal("expected error for expired auth_date")
	}
}

func TestVerifyTelegramLogin_ExtraFieldSigned(t *testing.T) {
	// Unknown-but-signed fields must still verify (Telegram may add fields).
	now := time.Now()
	fields := map[string]string{
		"id":         "42",
		"first_name": "A",
		"auth_date":  strconv.FormatInt(now.Unix(), 10),
		"some_new":   "value",
	}
	fields["hash"] = signFields(t, fields, testBotToken)
	if _, err := VerifyTelegramLogin(fields, testBotToken, now); err != nil {
		t.Fatalf("expected valid login with extra signed field, got: %v", err)
	}
}
