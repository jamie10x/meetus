package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

const miniAppBotToken = "123456:MINIAPP-TEST-TOKEN"

// signMiniAppFields signs fields using the real Mini App algorithm
// (independently reimplemented here) so tests can produce valid initData
// without depending on VerifyMiniAppInitData's own internals.
func signMiniAppFields(t *testing.T, fields map[string]string, botToken string) string {
	t.Helper()
	keys := make([]string, 0, len(fields))
	for k := range fields {
		if k != "hash" && k != "signature" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	lines := make([]string, 0, len(keys))
	for _, k := range keys {
		lines = append(lines, k+"="+fields[k])
	}
	dataCheckString := strings.Join(lines, "\n")

	secretMAC := hmac.New(sha256.New, []byte("WebAppData"))
	secretMAC.Write([]byte(botToken))
	secretKey := secretMAC.Sum(nil)

	mac := hmac.New(sha256.New, secretKey)
	mac.Write([]byte(dataCheckString))
	return hex.EncodeToString(mac.Sum(nil))
}

func validMiniAppInitData(t *testing.T, now time.Time) string {
	t.Helper()
	fields := map[string]string{
		"query_id":  "AAtest",
		"user":      `{"id":555,"first_name":"Aziza","last_name":"K","username":"aziza","language_code":"ru"}`,
		"auth_date": strconv.FormatInt(now.Unix(), 10),
	}
	fields["hash"] = signMiniAppFields(t, fields, miniAppBotToken)

	v := url.Values{}
	for k, val := range fields {
		v.Set(k, val)
	}
	return v.Encode()
}

func TestVerifyMiniAppInitData_Valid(t *testing.T) {
	now := time.Now()
	u, err := VerifyMiniAppInitData(validMiniAppInitData(t, now), miniAppBotToken, now)
	if err != nil {
		t.Fatalf("expected valid init data, got error: %v", err)
	}
	if u.ID != 555 {
		t.Errorf("ID = %d, want 555", u.ID)
	}
	if u.LanguageCode != "ru" {
		t.Errorf("LanguageCode = %q, want %q", u.LanguageCode, "ru")
	}
	if got := u.DisplayName(); got != "Aziza K" {
		t.Errorf("DisplayName = %q", got)
	}
}

func TestVerifyMiniAppInitData_TamperedField(t *testing.T) {
	now := time.Now()
	initData := validMiniAppInitData(t, now)
	tampered := strings.Replace(initData, "query_id=AAtest", "query_id=AAhack", 1)
	if tampered == initData {
		t.Fatal("substitution did not change the string")
	}
	if _, err := VerifyMiniAppInitData(tampered, miniAppBotToken, now); err == nil {
		t.Fatal("expected error for tampered init data")
	}
}

func TestVerifyMiniAppInitData_WrongBotToken(t *testing.T) {
	now := time.Now()
	initData := validMiniAppInitData(t, now)
	if _, err := VerifyMiniAppInitData(initData, "other:token", now); err == nil {
		t.Fatal("expected error for wrong bot token")
	}
}

func TestVerifyMiniAppInitData_ExpiredAuthDate(t *testing.T) {
	old := time.Now().Add(-2 * time.Hour)
	initData := validMiniAppInitData(t, old)
	if _, err := VerifyMiniAppInitData(initData, miniAppBotToken, time.Now()); err == nil {
		t.Fatal("expected error for expired auth_date")
	}
}

func TestVerifyMiniAppInitData_MissingHash(t *testing.T) {
	now := time.Now()
	v, _ := url.ParseQuery(validMiniAppInitData(t, now))
	v.Del("hash")
	if _, err := VerifyMiniAppInitData(v.Encode(), miniAppBotToken, now); err == nil {
		t.Fatal("expected error for missing hash")
	}
}

func TestVerifyMiniAppInitData_MalformedUser(t *testing.T) {
	now := time.Now()
	fields := map[string]string{
		"auth_date": strconv.FormatInt(now.Unix(), 10),
		"user":      "not-json",
	}
	fields["hash"] = signMiniAppFields(t, fields, miniAppBotToken)
	v := url.Values{}
	for k, val := range fields {
		v.Set(k, val)
	}
	if _, err := VerifyMiniAppInitData(v.Encode(), miniAppBotToken, now); err == nil {
		t.Fatal("expected error for malformed user field")
	}
}

// Confirms the Mini App secret derivation is genuinely different from the
// Login Widget's — a payload valid for one must not verify under the other.
func TestVerifyMiniAppInitData_NotInterchangeableWithLoginWidget(t *testing.T) {
	now := time.Now()
	widgetFields := map[string]string{
		"id":         "555",
		"first_name": "Aziza",
		"auth_date":  strconv.FormatInt(now.Unix(), 10),
	}
	widgetFields["hash"] = signFields(t, widgetFields, miniAppBotToken)
	v := url.Values{}
	for k, val := range widgetFields {
		v.Set(k, val)
	}
	if _, err := VerifyMiniAppInitData(v.Encode(), miniAppBotToken, now); err == nil {
		t.Fatal("a Login Widget signature must not verify as Mini App init data")
	}
}
