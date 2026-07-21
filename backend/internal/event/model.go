package event

import "time"

type Status string

const (
	StatusDraft     Status = "draft"
	StatusPublished Status = "published"
	StatusCanceled  Status = "canceled"
	StatusFinished  Status = "finished"
)

type Visibility string

const (
	VisibilityPublic   Visibility = "public"
	VisibilityUnlisted Visibility = "unlisted"
)

type Event struct {
	ID           int64
	OrganizerID  int64
	Title        string
	Description  string
	CategoryID   int32
	CityID       *int32
	District     *string
	LocationName *string
	Address      *string
	Lat          *float64
	Lng          *float64
	IsOnline     bool
	StartsAt     time.Time
	EndsAt       *time.Time
	Capacity     *int32
	CoverURL     *string
	Status       Status
	Visibility   Visibility
	SeriesID     *int64
	CreatedAt    time.Time
	UpdatedAt    time.Time

	// Denormalized fields loaded with joins where useful.
	OrganizerName     string
	OrganizerVerified bool
	CategorySlug      string
	CitySlug          *string
	GoingCount        int32
}
