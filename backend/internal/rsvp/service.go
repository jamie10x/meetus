package rsvp

import (
	"context"
	"log/slog"
	"time"

	"meetus.uz/backend/internal/event"
	"meetus.uz/backend/internal/platform/apperr"
	"meetus.uz/backend/internal/user"
)

// PromotionNotifier delivers a waitlist-promotion message (with ticket)
// to a newly-confirmed attendee. Satisfied by *tgbot.Announcer; declared
// here rather than importing tgbot directly so rsvp doesn't depend on
// it — tgbot already depends on rsvp, and Go disallows the cycle. Same
// pattern as channel.Announcer.
type PromotionNotifier interface {
	SendWaitlistPromotion(ctx context.Context, telegramID int64, langCode string, ticketCode string, e *event.Event) error
}

type Service struct {
	repo       *Repository
	signer     *TicketSigner
	events     *event.Repository
	users      *user.Repository
	onPromoted PromotionNotifier
}

func NewService(repo *Repository, signer *TicketSigner, events *event.Repository, users *user.Repository) *Service {
	return &Service{repo: repo, signer: signer, events: events, users: users}
}

// SetPromotionNotifier wires in the Telegram notification sent when a
// waitlisted attendee is promoted. Left unset (nil-safe), promotions
// still happen — the user just isn't messaged, which is fine for
// contexts (tests) that don't need it.
func (s *Service) SetPromotionNotifier(n PromotionNotifier) {
	s.onPromoted = n
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
// or "waitlisted" (without one yet).
type RSVPDTO struct {
	Status string     `json:"status"`
	Ticket *TicketDTO `json:"ticket"`
}

func (s *Service) rsvpDTO(status string, t *Ticket) RSVPDTO {
	dto := RSVPDTO{Status: status}
	if t != nil {
		td := s.ticketDTO(t)
		dto.Ticket = &td
	}
	return dto
}

func (s *Service) Join(ctx context.Context, eventID, userID int64) (RSVPDTO, error) {
	res, err := s.repo.Join(ctx, eventID, userID)
	if err != nil {
		return RSVPDTO{}, err
	}
	return s.rsvpDTO(res.Status, res.Ticket), nil
}

// Cancel cancels the caller's RSVP. If that frees a spot for a
// waitlisted attendee, the promotion notification is sent in the
// background — like the auto-announce-on-publish hook, on
// context.Background() rather than this request's context, since the
// request context is canceled the instant the HTTP response is written.
func (s *Service) Cancel(ctx context.Context, eventID, userID int64) error {
	promotion, err := s.repo.Cancel(ctx, eventID, userID)
	if err != nil {
		return err
	}
	if promotion != nil && s.onPromoted != nil {
		go s.notifyPromoted(context.Background(), promotion)
	}
	return nil
}

func (s *Service) notifyPromoted(ctx context.Context, p *Promotion) {
	e, err := s.events.GetByID(ctx, p.EventID)
	if err != nil {
		slog.Error("waitlist promotion: could not load event", "event_id", p.EventID, "err", err)
		return
	}
	u, err := s.users.GetByID(ctx, p.UserID)
	if err != nil {
		slog.Error("waitlist promotion: could not load user", "user_id", p.UserID, "err", err)
		return
	}
	if err := s.onPromoted.SendWaitlistPromotion(ctx, u.TelegramID, u.Language, p.Ticket.Code, e); err != nil {
		slog.Error("waitlist promotion notify failed", "user_id", p.UserID, "event_id", p.EventID, "err", err)
	}
}

func (s *Service) GetMine(ctx context.Context, eventID, userID int64) (RSVPDTO, error) {
	m, err := s.repo.GetMine(ctx, eventID, userID)
	if err != nil {
		return RSVPDTO{}, err
	}
	return s.rsvpDTO(m.Status, m.Ticket), nil
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
