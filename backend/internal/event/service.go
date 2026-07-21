package event

import (
	"context"
	"fmt"
	"time"

	"meetus.uz/backend/internal/platform/apperr"
)

// maxRecurWeeks caps a series at 12 total occurrences (~3 months
// weekly) — enough for a real recurring meetup, small enough that a
// mistaken input can't silently spawn a huge run of draft events.
const maxRecurWeeks = 11

type Service struct {
	repo *Repository
	now  func() time.Time
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo, now: time.Now}
}

type Input struct {
	Title        string   `json:"title" binding:"required,max=200"`
	Description  string   `json:"description" binding:"max=8000"`
	CategoryID   int32    `json:"categoryId" binding:"required"`
	CityID       *int32   `json:"cityId"`
	District     *string  `json:"district" binding:"omitempty,max=100"`
	LocationName *string  `json:"locationName" binding:"omitempty,max=200"`
	Address      *string  `json:"address" binding:"omitempty,max=300"`
	Lat          *float64 `json:"lat"`
	Lng          *float64 `json:"lng"`
	IsOnline     bool     `json:"isOnline"`
	StartsAt     string   `json:"startsAt" binding:"required"`
	EndsAt       *string  `json:"endsAt"`
	Capacity     *int32   `json:"capacity"`
	CoverURL     *string  `json:"coverUrl"`
	Visibility   *string  `json:"visibility"`
	// RecurWeeks, if set and > 0, creates this many *additional* weekly
	// occurrences alongside the one described by the rest of Input (so
	// RecurWeeks: 3 makes 4 events total, one week apart). Only
	// meaningful on create — Update has no use for it and ignores it.
	RecurWeeks *int `json:"recurWeeks"`
}

func (s *Service) validate(in Input) (WriteFields, error) {
	startsAt, err := time.Parse(time.RFC3339, in.StartsAt)
	if err != nil {
		return WriteFields{}, apperr.Validation("startsAt must be RFC3339, e.g. 2026-08-01T18:00:00+05:00")
	}
	var endsAt *string
	if in.EndsAt != nil && *in.EndsAt != "" {
		e, err := time.Parse(time.RFC3339, *in.EndsAt)
		if err != nil {
			return WriteFields{}, apperr.Validation("endsAt must be RFC3339")
		}
		if !e.After(startsAt) {
			return WriteFields{}, apperr.Validation("endsAt must be after startsAt")
		}
		endsAt = in.EndsAt
	}
	if in.Capacity != nil && *in.Capacity <= 0 {
		return WriteFields{}, apperr.Validation("capacity must be positive")
	}
	if in.RecurWeeks != nil && (*in.RecurWeeks < 0 || *in.RecurWeeks > maxRecurWeeks) {
		return WriteFields{}, apperr.Validation(fmt.Sprintf("recurWeeks must be between 0 and %d", maxRecurWeeks))
	}
	if !in.IsOnline && in.CityID == nil {
		return WriteFields{}, apperr.Validation("offline events require a cityId")
	}

	visibility := VisibilityPublic
	if in.Visibility != nil {
		switch Visibility(*in.Visibility) {
		case VisibilityPublic, VisibilityUnlisted:
			visibility = Visibility(*in.Visibility)
		default:
			return WriteFields{}, apperr.Validation("visibility must be public or unlisted")
		}
	}

	return WriteFields{
		Title:        in.Title,
		Description:  in.Description,
		CategoryID:   in.CategoryID,
		CityID:       in.CityID,
		District:     in.District,
		LocationName: in.LocationName,
		Address:      in.Address,
		Lat:          in.Lat,
		Lng:          in.Lng,
		IsOnline:     in.IsOnline,
		StartsAt:     in.StartsAt,
		EndsAt:       endsAt,
		Capacity:     in.Capacity,
		CoverURL:     in.CoverURL,
		Visibility:   visibility,
	}, nil
}

// Create makes one event, or — when in.RecurWeeks is set and positive —
// a whole weekly series at once. Either way it returns the first (or
// only) event; the API's create response has always been "the event
// just created," and for a series that's the first occurrence, with the
// rest visible via ListMine / the series' own SeriesID.
func (s *Service) Create(ctx context.Context, organizerID int64, in Input) (*Event, error) {
	fields, err := s.validate(in)
	if err != nil {
		return nil, err
	}
	if in.RecurWeeks != nil && *in.RecurWeeks > 0 {
		events, err := s.repo.CreateSeries(ctx, organizerID, fields, *in.RecurWeeks+1)
		if err != nil {
			return nil, err
		}
		return events[0], nil
	}
	return s.repo.Create(ctx, organizerID, fields)
}

// getOwned loads an event and verifies it belongs to the organizer.
func (s *Service) getOwned(ctx context.Context, organizerID, eventID int64) (*Event, error) {
	e, err := s.repo.GetByID(ctx, eventID)
	if err != nil {
		return nil, err
	}
	if e.OrganizerID != organizerID {
		return nil, apperr.Forbidden("you do not own this event")
	}
	return e, nil
}

func (s *Service) Update(ctx context.Context, organizerID, eventID int64, in Input) (*Event, error) {
	e, err := s.getOwned(ctx, organizerID, eventID)
	if err != nil {
		return nil, err
	}
	if e.Status == StatusCanceled || e.Status == StatusFinished {
		return nil, apperr.Conflict("canceled or finished events cannot be edited")
	}
	fields, err := s.validate(in)
	if err != nil {
		return nil, err
	}
	return s.repo.Update(ctx, eventID, fields)
}

func (s *Service) Publish(ctx context.Context, organizerID, eventID int64) (*Event, error) {
	e, err := s.getOwned(ctx, organizerID, eventID)
	if err != nil {
		return nil, err
	}
	if e.Status != StatusDraft {
		return nil, apperr.Conflict("only drafts can be published")
	}
	if !e.StartsAt.After(s.now()) {
		return nil, apperr.Validation("cannot publish an event that starts in the past")
	}
	if err := s.repo.SetStatus(ctx, eventID, StatusPublished); err != nil {
		return nil, err
	}
	return s.repo.GetByID(ctx, eventID)
}

func (s *Service) Unpublish(ctx context.Context, organizerID, eventID int64) (*Event, error) {
	e, err := s.getOwned(ctx, organizerID, eventID)
	if err != nil {
		return nil, err
	}
	if e.Status != StatusPublished {
		return nil, apperr.Conflict("only published events can be unpublished")
	}
	if err := s.repo.SetStatus(ctx, eventID, StatusDraft); err != nil {
		return nil, err
	}
	return s.repo.GetByID(ctx, eventID)
}

func (s *Service) Cancel(ctx context.Context, organizerID, eventID int64) (*Event, error) {
	e, err := s.getOwned(ctx, organizerID, eventID)
	if err != nil {
		return nil, err
	}
	if e.Status == StatusCanceled || e.Status == StatusFinished {
		return nil, apperr.Conflict("event is already " + string(e.Status))
	}
	if err := s.repo.SetStatus(ctx, eventID, StatusCanceled); err != nil {
		return nil, err
	}
	return s.repo.GetByID(ctx, eventID)
}

func (s *Service) Delete(ctx context.Context, organizerID, eventID int64) error {
	e, err := s.getOwned(ctx, organizerID, eventID)
	if err != nil {
		return err
	}
	if e.Status != StatusDraft {
		return apperr.Conflict("only drafts can be deleted; cancel the event instead")
	}
	return s.repo.Delete(ctx, eventID)
}

func (s *Service) ListMine(ctx context.Context, organizerID int64) ([]*Event, error) {
	return s.repo.ListByOrganizer(ctx, organizerID)
}
