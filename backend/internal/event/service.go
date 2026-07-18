package event

import (
	"context"
	"time"

	"meetus.uz/backend/internal/platform/apperr"
)

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

func (s *Service) Create(ctx context.Context, organizerID int64, in Input) (*Event, error) {
	fields, err := s.validate(in)
	if err != nil {
		return nil, err
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
