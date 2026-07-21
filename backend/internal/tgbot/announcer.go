package tgbot

import (
	"context"
	"fmt"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"meetus.uz/backend/internal/event"
	"meetus.uz/backend/internal/rsvp"
)

// Announcer sends one-off channel announcement and RSVP-notification
// messages. Unlike Bot, it never polls for updates (no Start() call) —
// it's constructed once in the API process alongside the other shared
// dependencies in server/router.go, purely to call SendMessage when an
// organizer clicks "Announce" on a published event, or when a waitlisted
// attendee gets promoted after someone else cancels. bot.WithSkipGetMe
// skips the network round trip Bot.New would otherwise pay on every API
// server boot.
type Announcer struct {
	api        *bot.Bot
	webBaseURL string
	loc        *time.Location
	signer     *rsvp.TicketSigner
}

func NewAnnouncer(token, webBaseURL string, signer *rsvp.TicketSigner) (*Announcer, error) {
	api, err := bot.New(token, bot.WithSkipGetMe())
	if err != nil {
		return nil, fmt.Errorf("create telegram announcer: %w", err)
	}
	return &Announcer{api: api, webBaseURL: webBaseURL, loc: tashkentLocation(), signer: signer}, nil
}

// SendWaitlistPromotion notifies a waitlisted attendee that a spot has
// opened up and their RSVP is now confirmed, with their ticket QR
// attached — the same rendering Bot uses for a fresh join, since from
// the attendee's point of view this *is* their ticket arriving.
// Satisfies rsvp.PromotionNotifier.
func (a *Announcer) SendWaitlistPromotion(ctx context.Context, telegramID int64, langCode string, ticketCode string, e *event.Event) error {
	l := normalizeLang(langCode)
	text := tf(l, kWaitlistPromoted, escape(e.Title))
	if _, err := a.api.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    telegramID,
		Text:      text,
		ParseMode: models.ParseModeHTML,
	}); err != nil {
		return fmt.Errorf("send promotion message: %w", err)
	}
	return sendTicketPhotoTo(ctx, a.api, a.loc, a.signer, telegramID, l, ticketCode, e)
}

// SendAnnouncement posts one event to one connected channel, rendered in
// langCode — the channel's own language override if the caller set one,
// else the triggering organizer's language.
func (a *Announcer) SendAnnouncement(ctx context.Context, chatID int64, langCode string, e *event.Event) error {
	l := normalizeLang(langCode)

	text := fmt.Sprintf("📢 <b>%s</b>\n\n🕐 %s\n📍 %s\n\n👤 %s",
		escape(e.Title), formatEventTime(l, e.StartsAt, a.loc), escape(eventPlaceLabel(l, e)), escape(e.OrganizerName))

	// A plain URL button, not WebApp: Telegram only allows web_app buttons
	// in private chats — using one in a channel post gets the whole send
	// rejected with BUTTON_TYPE_INVALID. The link still opens the Mini App
	// experience fine via Telegram's in-app browser, just without the
	// native web_app launch.
	markup := &models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{
		{{Text: t(l, kAnnouncementCta), URL: buildWebURL(a.webBaseURL, l, fmt.Sprintf("/events/%d", e.ID))}},
	}}

	_, err := a.api.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      chatID,
		Text:        text,
		ParseMode:   models.ParseModeHTML,
		ReplyMarkup: markup,
	})
	return err
}
