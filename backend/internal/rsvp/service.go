package rsvp

import (
	"context"
	"time"

	"meetus.uz/backend/internal/platform/apperr"
)

type Service struct {
	repo   *Repository
	signer *TicketSigner
}

func NewService(repo *Repository, signer *TicketSigner) *Service {
	return &Service{repo: repo, signer: signer}
}

type TicketDTO struct {
	Code        string     `json:"code"`
	QR          string     `json:"qr"`
	CheckedInAt *time.Time `json:"checkedInAt"`
}

func (s *Service) ticketDTO(t *Ticket) TicketDTO {
	return TicketDTO{Code: t.Code, QR: s.signer.QRValue(t.Code), CheckedInAt: t.CheckedInAt}
}

func (s *Service) Join(ctx context.Context, eventID, userID int64) (TicketDTO, error) {
	t, err := s.repo.Join(ctx, eventID, userID)
	if err != nil {
		return TicketDTO{}, err
	}
	return s.ticketDTO(t), nil
}

func (s *Service) Cancel(ctx context.Context, eventID, userID int64) error {
	return s.repo.Cancel(ctx, eventID, userID)
}

func (s *Service) GetMine(ctx context.Context, eventID, userID int64) (TicketDTO, error) {
	t, err := s.repo.GetMine(ctx, eventID, userID)
	if err != nil {
		return TicketDTO{}, err
	}
	return s.ticketDTO(t), nil
}

type MyTicketDTO struct {
	TicketDTO
	EventID      int64      `json:"eventId"`
	EventTitle   string     `json:"eventTitle"`
	EventStatus  string     `json:"eventStatus"`
	StartsAt     time.Time  `json:"startsAt"`
	IsOnline     bool       `json:"isOnline"`
	LocationName *string    `json:"locationName"`
	CitySlug     *string    `json:"citySlug"`
	CoverURL     *string    `json:"coverUrl"`
}

func (s *Service) ListMyTickets(ctx context.Context, userID int64) ([]MyTicketDTO, error) {
	tickets, err := s.repo.ListMyTickets(ctx, userID)
	if err != nil {
		return nil, err
	}
	dtos := make([]MyTicketDTO, len(tickets))
	for i, t := range tickets {
		dtos[i] = MyTicketDTO{
			TicketDTO:    s.ticketDTO(&t.Ticket),
			EventID:      t.EventID,
			EventTitle:   t.EventTitle,
			EventStatus:  t.EventStatus,
			StartsAt:     t.StartsAt,
			IsOnline:     t.IsOnline,
			LocationName: t.LocationName,
			CitySlug:     t.CitySlug,
			CoverURL:     t.CoverURL,
		}
	}
	return dtos, nil
}

type CheckInResult struct {
	AttendeeName string    `json:"attendeeName"`
	EventTitle   string    `json:"eventTitle"`
	CheckedInAt  time.Time `json:"checkedInAt"`
}

// CheckIn verifies a scanned QR, authorizes the organizer, and marks the
// ticket as used exactly once.
func (s *Service) CheckIn(ctx context.Context, organizerID int64, qr string) (*CheckInResult, error) {
	code, err := s.signer.VerifyQR(qr)
	if err != nil {
		return nil, err
	}
	t, err := s.repo.GetByCode(ctx, code)
	if err != nil {
		return nil, err
	}
	if t.EventOrganizerID != organizerID {
		return nil, apperr.Forbidden("this ticket belongs to another organizer's event")
	}
	if t.RSVPStatus != "going" {
		return nil, apperr.Conflict("this RSVP was canceled")
	}
	if t.CheckedInAt != nil {
		return nil, apperr.Conflict("ticket already checked in at " +
			t.CheckedInAt.Format("15:04"))
	}
	at, err := s.repo.MarkCheckedIn(ctx, t.TicketID)
	if err != nil {
		return nil, err
	}
	return &CheckInResult{
		AttendeeName: t.AttendeeName,
		EventTitle:   t.EventTitle,
		CheckedInAt:  at,
	}, nil
}
