package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"meetus.uz/backend/internal/platform/apperr"
)

// miniAppAuthMaxAge is tighter than the Login Widget's window: initData
// is minted fresh by the Telegram client every time the Mini App opens,
// so there's no legitimate reason for it to be old.
const miniAppAuthMaxAge = time.Hour

// webAppDataConstant is the fixed key Telegram specifies for deriving the
// Mini App secret — see VerifyMiniAppInitData.
const webAppDataConstant = "WebAppData"

type miniAppUser struct {
	ID           int64  `json:"id"`
	FirstName    string `json:"first_name"`
	LastName     string `json:"last_name"`
	Username     string `json:"username"`
	PhotoURL     string `json:"photo_url"`
	LanguageCode string `json:"language_code"`
}

// VerifyMiniAppInitData validates the raw initData string a Telegram Mini
// App receives on launch (window.Telegram.WebApp.initData).
//
// This is a DIFFERENT signing scheme from the Login Widget
// (VerifyTelegramLogin), not a variant of it:
//   - data-check-string: same idea (sorted "key=value" lines, "\n"-joined),
//     but built from initData's fields, excluding both "hash" and
//     "signature" (the latter belongs to a separate, newer verification
//     scheme this function does not implement).
//   - secret key: HMAC-SHA256(key="WebAppData", message=botToken) — the
//     Login Widget instead uses a plain SHA-256 of the bot token. Do not
//     share the derived secret between the two flows.
//   - final hash: HMAC-SHA256(key=secretKey, message=dataCheckString),
//     same as the widget once you have the right secret.
func VerifyMiniAppInitData(initData string, botToken string, now time.Time) (*TelegramUser, error) {
	values, err := url.ParseQuery(initData)
	if err != nil {
		return nil, apperr.Unauthorized("malformed init data")
	}

	gotHash := values.Get("hash")
	if gotHash == "" {
		return nil, apperr.Unauthorized("missing init data hash")
	}

	keys := make([]string, 0, len(values))
	for k := range values {
		if k != "hash" && k != "signature" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)

	lines := make([]string, 0, len(keys))
	for _, k := range keys {
		lines = append(lines, k+"="+values.Get(k))
	}
	dataCheckString := strings.Join(lines, "\n")

	secretMAC := hmac.New(sha256.New, []byte(webAppDataConstant))
	secretMAC.Write([]byte(botToken))
	secretKey := secretMAC.Sum(nil)

	mac := hmac.New(sha256.New, secretKey)
	mac.Write([]byte(dataCheckString))
	wantHash := hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(wantHash), []byte(gotHash)) {
		return nil, apperr.Unauthorized("invalid init data signature")
	}

	authDate, err := strconv.ParseInt(values.Get("auth_date"), 10, 64)
	if err != nil {
		return nil, apperr.Unauthorized("invalid init data auth_date")
	}
	if now.Sub(time.Unix(authDate, 0)) > miniAppAuthMaxAge {
		return nil, apperr.Unauthorized("init data expired, please relaunch the app")
	}

	var mu miniAppUser
	if err := json.Unmarshal([]byte(values.Get("user")), &mu); err != nil || mu.ID <= 0 {
		return nil, apperr.Unauthorized("init data missing user")
	}
	if mu.FirstName == "" {
		return nil, apperr.Unauthorized("init data user missing first_name")
	}

	return &TelegramUser{
		ID:           mu.ID,
		FirstName:    mu.FirstName,
		LastName:     mu.LastName,
		Username:     mu.Username,
		PhotoURL:     mu.PhotoURL,
		LanguageCode: mu.LanguageCode,
	}, nil
}
