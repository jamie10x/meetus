package user

import "time"

type DTO struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Username  *string   `json:"username"`
	AvatarURL *string   `json:"avatarUrl"`
	CityID    *int32    `json:"cityId"`
	District  *string   `json:"district"`
	Language  string    `json:"language"`
	CreatedAt time.Time `json:"createdAt"`
}

func (u *User) ToDTO() DTO {
	return DTO{
		ID:        u.ID,
		Name:      u.Name,
		Username:  u.Username,
		AvatarURL: u.AvatarURL,
		CityID:    u.CityID,
		District:  u.District,
		Language:  u.Language,
		CreatedAt: u.CreatedAt,
	}
}
