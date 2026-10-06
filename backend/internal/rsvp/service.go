package rsvp

import (
	"context"
	"time"
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

// RSVPDTO is the caller's RSVP outcome or state: "going" (with a ticket)
// or "waitlisted" (without one yet). OnlineURL is only ever populated when
// Status is "going" — a waitlisted caller has no confirmed spot and must
// not receive the meeting link (rsvpDTO enforces this, not the callers).
type RSVPDTO struct {
	Status    string     `json:"status"`
	Ticket    *TicketDTO `json:"ticket"`
	OnlineURL *string    `json:"onlineUrl"`
}

func (s *Service) rsvpDTO(status string, t *Ticket, onlineURL *string) RSVPDTO {
	dto := RSVPDTO{Status: status}
	if t != nil {
		td := s.ticketDTO(t)
		dto.Ticket = &td
	}
	if status == "going" {
		dto.OnlineURL = onlineURL
	}
	return dto
}

func (s *Service) Join(ctx context.Context, eventID, userID int64) (RSVPDTO, error) {
	res, err := s.repo.Join(ctx, eventID, userID)
	if err != nil {
		return RSVPDTO{}, err
	}
	return s.rsvpDTO(res.Status, res.Ticket, res.OnlineURL), nil
}

// Cancel commits the RSVP change and any promotion delivery job atomically.
func (s *Service) Cancel(ctx context.Context, eventID, userID int64) error {
	_, err := s.repo.Cancel(ctx, eventID, userID)
	return err
}

func (s *Service) GetMine(ctx context.Context, eventID, userID int64) (RSVPDTO, error) {
	m, err := s.repo.GetMine(ctx, eventID, userID)
	if err != nil {
		return RSVPDTO{}, err
	}
	return s.rsvpDTO(m.Status, m.Ticket, m.OnlineURL), nil
}

// MyTicketDTO's OnlineURL is always safe to include as-is (unlike
// RSVPDTO's) — every row ListMyTickets returns is inherently a confirmed
// "going" RSVP; a waitlisted entry has no ticket and never appears here.
type MyTicketDTO struct {
	TicketDTO
	EventID      int64     `json:"eventId"`
	EventTitle   string    `json:"eventTitle"`
	EventStatus  string    `json:"eventStatus"`
	StartsAt     time.Time `json:"startsAt"`
	IsOnline     bool      `json:"isOnline"`
	OnlineURL    *string   `json:"onlineUrl"`
	LocationName *string   `json:"locationName"`
	CitySlug     *string   `json:"citySlug"`
	CoverURL     *string   `json:"coverUrl"`
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
			OnlineURL:    t.OnlineURL,
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
func (s *Service) CheckIn(ctx context.Context, organizerID, eventID int64, qr string) (*CheckInResult, error) {
	code, err := s.signer.VerifyQR(qr)
	if err != nil {
		return nil, err
	}
	return s.repo.CheckIn(ctx, organizerID, eventID, code)
}
