// Package tglang maps a Telegram client's IETF language_code to one of
// the three languages Meetus supports. Shared by the bot (from Update.From)
// and Mini App login (from initData's user field) so both first-contact
// paths guess a new user's language the same way.
package tglang

import "strings"

// MapCode returns "uz", "ru", or "en". Unrecognized codes default to "uz"
// (majority audience), matching the users.language column default.
func MapCode(code string) string {
	c := strings.ToLower(code)
	switch {
	case strings.HasPrefix(c, "ru"):
		return "ru"
	case strings.HasPrefix(c, "en"):
		return "en"
	default:
		return "uz"
	}
}
