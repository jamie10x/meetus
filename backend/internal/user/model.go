package user

import "time"

type User struct {
	ID                  int64
	TelegramID          int64
	Name                string
	Username            *string
	AvatarURL           *string
	CityID              *int32
	District            *string
	Language            string
	IsBanned            bool
	IsAdmin             bool
	NotificationsMuted  bool
	WeeklyDigestEnabled bool
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

// TelegramProfile is the identity data received from Telegram login,
// used to create or refresh the user row.
type TelegramProfile struct {
	TelegramID int64
	Name       string
	Username   string
	AvatarURL  string
	// Language seeds the language column on first insert only — it is
	// never applied to an existing user, since they may have since
	// changed it via /language or the web profile page. Callers without
	// a language hint (e.g. web login) should pass "uz", matching the
	// column's own default.
	Language string
}
