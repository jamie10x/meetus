package user

import "time"

type User struct {
	ID         int64
	TelegramID int64
	Name       string
	Username   *string
	AvatarURL  *string
	CityID     *int32
	District   *string
	Language   string
	IsBanned   bool
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// TelegramProfile is the identity data received from Telegram login,
// used to create or refresh the user row.
type TelegramProfile struct {
	TelegramID int64
	Name       string
	Username   string
	AvatarURL  string
}
