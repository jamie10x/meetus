package event

import "time"

type DTO struct {
	ID            int64      `json:"id"`
	OrganizerID   int64      `json:"organizerId"`
	OrganizerName string     `json:"organizerName"`
	Title         string     `json:"title"`
	Description   string     `json:"description"`
	CategoryID    int32      `json:"categoryId"`
	CategorySlug  string     `json:"categorySlug"`
	CityID        *int32     `json:"cityId"`
	CitySlug      *string    `json:"citySlug"`
	District      *string    `json:"district"`
	LocationName  *string    `json:"locationName"`
	Address       *string    `json:"address"`
	Lat           *float64   `json:"lat"`
	Lng           *float64   `json:"lng"`
	IsOnline      bool       `json:"isOnline"`
	StartsAt      time.Time  `json:"startsAt"`
	EndsAt        *time.Time `json:"endsAt"`
	Capacity      *int32     `json:"capacity"`
	CoverURL      *string    `json:"coverUrl"`
	Status        string     `json:"status"`
	Visibility    string     `json:"visibility"`
	GoingCount    int32      `json:"goingCount"`
	CreatedAt     time.Time  `json:"createdAt"`
}

func (e *Event) ToDTO() DTO {
	return DTO{
		ID:            e.ID,
		OrganizerID:   e.OrganizerID,
		OrganizerName: e.OrganizerName,
		Title:         e.Title,
		Description:   e.Description,
		CategoryID:    e.CategoryID,
		CategorySlug:  e.CategorySlug,
		CityID:        e.CityID,
		CitySlug:      e.CitySlug,
		District:      e.District,
		LocationName:  e.LocationName,
		Address:       e.Address,
		Lat:           e.Lat,
		Lng:           e.Lng,
		IsOnline:      e.IsOnline,
		StartsAt:      e.StartsAt,
		EndsAt:        e.EndsAt,
		Capacity:      e.Capacity,
		CoverURL:      e.CoverURL,
		Status:        string(e.Status),
		Visibility:    string(e.Visibility),
		GoingCount:    e.GoingCount,
		CreatedAt:     e.CreatedAt,
	}
}
