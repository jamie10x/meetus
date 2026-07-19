package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"meetus.uz/backend/internal/platform/apperr"
)

// telegramAuthMaxAge rejects login payloads older than this to limit replay.
const telegramAuthMaxAge = 24 * time.Hour

type TelegramUser struct {
	ID        int64
	FirstName string
	LastName  string
	Username  string
	PhotoURL  string
	// LanguageCode is only ever populated by Mini App login (initData's
	// user.language_code) — the Login Widget payload has no such field.
	LanguageCode string
}

// VerifyTelegramLogin validates a payload from the Telegram Login Widget.
//
// Algorithm (per Telegram docs): the data-check-string is every received
// field except "hash", sorted alphabetically and joined as "key=value" lines;
// the secret key is SHA256(botToken); the payload is valid when
// HMAC-SHA256(dataCheckString, secretKey) equals the "hash" field.
func VerifyTelegramLogin(fields map[string]string, botToken string, now time.Time) (*TelegramUser, error) {
	gotHash, ok := fields["hash"]
	if !ok || gotHash == "" {
		return nil, apperr.Unauthorized("missing telegram hash")
	}

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
	dataCheckString := strings.Join(lines, "\n")

	secret := sha256.Sum256([]byte(botToken))
	mac := hmac.New(sha256.New, secret[:])
	mac.Write([]byte(dataCheckString))
	wantHash := hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(wantHash), []byte(gotHash)) {
		return nil, apperr.Unauthorized("invalid telegram signature")
	}

	authDate, err := strconv.ParseInt(fields["auth_date"], 10, 64)
	if err != nil {
		return nil, apperr.Unauthorized("invalid telegram auth_date")
	}
	if now.Sub(time.Unix(authDate, 0)) > telegramAuthMaxAge {
		return nil, apperr.Unauthorized("telegram login expired, please sign in again")
	}

	id, err := strconv.ParseInt(fields["id"], 10, 64)
	if err != nil || id <= 0 {
		return nil, apperr.Unauthorized("invalid telegram user id")
	}

	tu := &TelegramUser{
		ID:        id,
		FirstName: fields["first_name"],
		LastName:  fields["last_name"],
		Username:  fields["username"],
		PhotoURL:  fields["photo_url"],
	}
	if tu.FirstName == "" {
		return nil, apperr.Unauthorized("telegram payload missing first_name")
	}
	return tu, nil
}

// DisplayName joins first and last name for storage.
func (u *TelegramUser) DisplayName() string {
	if u.LastName == "" {
		return u.FirstName
	}
	return fmt.Sprintf("%s %s", u.FirstName, u.LastName)
}
