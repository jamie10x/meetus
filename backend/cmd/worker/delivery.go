package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	telegram "github.com/go-telegram/bot"
	"github.com/jackc/pgx/v5/pgxpool"
	"meetus.uz/backend/internal/channel"
	"meetus.uz/backend/internal/config"
	"meetus.uz/backend/internal/event"
	"meetus.uz/backend/internal/groupfeed"
	"meetus.uz/backend/internal/notification"
	"meetus.uz/backend/internal/organizer"
	"meetus.uz/backend/internal/platform/apperr"
	"meetus.uz/backend/internal/platform/outbox"
	"meetus.uz/backend/internal/rsvp"
	"meetus.uz/backend/internal/tgbot"
	"meetus.uz/backend/internal/user"
	"time"
)

type announcement struct {
	EventID  int64  `json:"eventId"`
	ChatID   int64  `json:"chatId"`
	Language string `json:"language"`
}

func deliveryHandler(pool *pgxpool.Pool, cfg *config.Config, b *tgbot.Bot, a *tgbot.Announcer) func(context.Context, outbox.Job) error {
	events := event.NewRepository(pool)
	users := user.NewRepository(pool)
	channels := channel.NewRepository(pool)
	groups := groupfeed.NewRepository(pool)
	orgs := organizer.NewRepository(pool)
	active := func(ctx context.Context, id int64) (*user.User, error) {
		u, err := users.GetByID(ctx, id)
		if err != nil {
			return nil, err
		}
		if u.IsBanned || u.NotificationsMuted {
			return nil, nil
		}
		return u, nil
	}
	deliver := func(ctx context.Context, j outbox.Job) error {
		switch j.Kind {
		case "publish":
			var payload announcement
			if err := json.Unmarshal(j.Payload, &payload); err != nil {
				return outbox.Permanent(err)
			}
			e, err := events.GetByID(ctx, payload.EventID)
			if err != nil {
				return err
			}
			if e.Status != event.StatusPublished {
				return nil
			}
			enqueue := func(chatID int64, language string) error {
				return outbox.Enqueue(ctx, pool, fmt.Sprintf("announce:%d:%d", j.ID, chatID), "announcement", announcement{e.ID, chatID, language})
			}
			// Official feed remains independent of organizer channel configuration.
			if cfg.OfficialChannelID != 0 {
				if err = enqueue(cfg.OfficialChannelID, cfg.OfficialChannelLanguage); err != nil {
					return err
				}
			}
			chatIDs, err := groups.ListChatIDs(ctx)
			if err != nil {
				return err
			}
			for _, id := range chatIDs {
				if err = enqueue(id, cfg.OfficialChannelLanguage); err != nil {
					return err
				}
			}
			connected, err := channels.ListForOrganizer(ctx, e.OrganizerID)
			if err != nil {
				return err
			}
			if len(connected) == 0 {
				return nil
			}
			language, err := orgs.GetLanguage(ctx, e.OrganizerID)
			if err != nil {
				return err
			}
			for _, ch := range connected {
				if ch.ChatID == cfg.OfficialChannelID {
					continue
				}
				lang := language
				if ch.Language != nil {
					lang = *ch.Language
				}
				if err = enqueue(ch.ChatID, lang); err != nil {
					return err
				}
			}
			return nil
		case "announcement":
			var p announcement
			if err := json.Unmarshal(j.Payload, &p); err != nil {
				return outbox.Permanent(err)
			}
			e, err := events.GetByID(ctx, p.EventID)
			if err != nil {
				return err
			}
			if e.Status != event.StatusPublished {
				return nil
			}
			return a.SendAnnouncement(ctx, p.ChatID, p.Language, e)
		case "promotion":
			var p rsvp.Promotion
			if err := json.Unmarshal(j.Payload, &p); err != nil {
				return outbox.Permanent(err)
			}
			u, err := users.GetByID(ctx, p.UserID)
			if err != nil {
				return err
			}
			if u.IsBanned {
				return nil
			}
			mine, err := rsvp.NewRepository(pool).GetMine(ctx, p.EventID, p.UserID)
			if err != nil {
				return err
			}
			if mine.Status != "going" || mine.Ticket == nil || p.Ticket == nil || mine.Ticket.Code != p.Ticket.Code {
				return nil
			}
			e, err := events.GetByID(ctx, p.EventID)
			if err != nil {
				return err
			}
			if e.Status != event.StatusPublished || !e.StartsAt.After(time.Now()) {
				return nil
			}
			return a.SendWaitlistPromotion(ctx, u.TelegramID, u.Language, mine.Ticket.Code, e)
		case "reminder":
			var p notification.Reminder
			if err := json.Unmarshal(j.Payload, &p); err != nil {
				return outbox.Permanent(err)
			}
			u, err := active(ctx, p.UserID)
			if err != nil || u == nil {
				return err
			}
			e, err := events.GetByID(ctx, p.EventID)
			if err != nil {
				return err
			}
			if e.Status != event.StatusPublished || !e.StartsAt.After(time.Now()) {
				return nil
			}
			mine, err := rsvp.NewRepository(pool).GetMine(ctx, p.EventID, p.UserID)
			if err != nil {
				return err
			}
			if mine.Status != "going" {
				return nil
			}
			p.EventTitle = e.Title
			p.StartsAt = e.StartsAt
			p.LocationName = e.LocationName
			p.CitySlug = e.CitySlug
			p.IsOnline = e.IsOnline
			p.UserLanguage = u.Language
			return b.SendReminder(ctx, &p)
		case "feedback":
			var p notification.FeedbackDue
			if err := json.Unmarshal(j.Payload, &p); err != nil {
				return outbox.Permanent(err)
			}
			u, err := active(ctx, p.UserID)
			if err != nil || u == nil {
				return err
			}
			p.UserLanguage = u.Language
			return b.SendFeedbackRequest(ctx, &p)
		case "digest-batch":
			subscribers, err := users.ListWeeklyDigestSubscribers(ctx)
			if err != nil {
				return err
			}
			for _, sub := range subscribers {
				if err = outbox.Enqueue(ctx, pool, fmt.Sprintf("digest:%d:%d", j.ID, sub.UserID), "digest", sub); err != nil {
					return err
				}
			}
			return nil
		case "digest":
			var p user.DigestSubscriber
			if err := json.Unmarshal(j.Payload, &p); err != nil {
				return outbox.Permanent(err)
			}
			u, err := active(ctx, p.UserID)
			if err != nil || u == nil {
				return err
			}
			if !u.WeeklyDigestEnabled {
				return nil
			}
			p.CityID = u.CityID
			p.Language = u.Language
			return b.SendWeeklyDigest(ctx, &p)
		default:
			return outbox.Permanent(fmt.Errorf("unknown delivery kind"))
		}
	}
	return func(ctx context.Context, j outbox.Job) error {
		err := deliver(ctx, j)
		var app *apperr.Error
		if errors.Is(err, telegram.ErrorForbidden) || errors.Is(err, telegram.ErrorBadRequest) || errors.Is(err, telegram.ErrorUnauthorized) || errors.As(err, &app) && app.Code == apperr.CodeNotFound {
			return outbox.Permanent(err)
		}
		return err
	}
}
