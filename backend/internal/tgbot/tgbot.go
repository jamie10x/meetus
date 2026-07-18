// Package tgbot implements the Meetus Telegram bot: browsing events,
// one-tap RSVP, and reminder delivery. Users are identified by their
// Telegram ID — the same identity used for website login.
package tgbot

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"meetus.uz/backend/internal/event"
	"meetus.uz/backend/internal/notification"
	"meetus.uz/backend/internal/rsvp"
	"meetus.uz/backend/internal/user"
)

const browseLimit = 6

type Bot struct {
	api        *bot.Bot
	users      *user.Repository
	events     *event.Repository
	rsvps      *rsvp.Repository
	webBaseURL string
	loc        *time.Location
}

func New(token string, users *user.Repository, events *event.Repository, rsvps *rsvp.Repository, webBaseURL string) (*Bot, error) {
	b := &Bot{
		users:      users,
		events:     events,
		rsvps:      rsvps,
		webBaseURL: webBaseURL,
	}
	loc, err := time.LoadLocation("Asia/Tashkent")
	if err != nil {
		loc = time.FixedZone("UZT", 5*3600)
	}
	b.loc = loc

	api, err := bot.New(token, bot.WithDefaultHandler(b.handleDefault))
	if err != nil {
		return nil, fmt.Errorf("create telegram bot: %w", err)
	}
	b.api = api

	api.RegisterHandler(bot.HandlerTypeMessageText, "/start", bot.MatchTypePrefix, b.handleStart)
	api.RegisterHandler(bot.HandlerTypeMessageText, "/events", bot.MatchTypePrefix, b.handleEvents)
	api.RegisterHandler(bot.HandlerTypeCallbackQueryData, "ev:", bot.MatchTypePrefix, b.handleEventDetail)
	api.RegisterHandler(bot.HandlerTypeCallbackQueryData, "join:", bot.MatchTypePrefix, b.handleJoin)
	return b, nil
}

// Start runs long polling until ctx is canceled.
func (b *Bot) Start(ctx context.Context) {
	slog.Info("telegram bot polling started")
	b.api.Start(ctx)
}

// upsertUser registers or refreshes the Telegram user and returns the
// internal user record.
func (b *Bot) upsertUser(ctx context.Context, from *models.User) (*user.User, error) {
	name := from.FirstName
	if from.LastName != "" {
		name += " " + from.LastName
	}
	return b.users.UpsertTelegramUser(ctx, user.TelegramProfile{
		TelegramID: from.ID,
		Name:       name,
		Username:   from.Username,
	})
}

func (b *Bot) send(ctx context.Context, chatID int64, text string, markup models.ReplyMarkup) {
	_, err := b.api.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      chatID,
		Text:        text,
		ParseMode:   models.ParseModeHTML,
		ReplyMarkup: markup,
	})
	if err != nil {
		slog.Error("bot send failed", "chat_id", chatID, "err", err)
	}
}

func (b *Bot) handleStart(ctx context.Context, _ *bot.Bot, update *models.Update) {
	msg := update.Message
	if msg == nil || msg.From == nil {
		return
	}
	if _, err := b.upsertUser(ctx, msg.From); err != nil {
		slog.Error("bot start upsert failed", "err", err)
	}
	b.send(ctx, msg.Chat.ID,
		fmt.Sprintf("👋 Welcome to <b>Meetus.uz</b>, %s!\n\n"+
			"Discover meetups across Uzbekistan and join with one tap.\n\n"+
			"• /events — upcoming events\n"+
			"• Tickets and profile: %s", msg.From.FirstName, b.webBaseURL),
		nil)
}

func (b *Bot) handleDefault(ctx context.Context, _ *bot.Bot, update *models.Update) {
	if update.Message == nil || update.Message.From == nil {
		return
	}
	b.send(ctx, update.Message.Chat.ID,
		"Try /events to browse upcoming meetups, or visit "+b.webBaseURL, nil)
}

func (b *Bot) handleEvents(ctx context.Context, _ *bot.Bot, update *models.Update) {
	msg := update.Message
	if msg == nil || msg.From == nil {
		return
	}
	u, err := b.upsertUser(ctx, msg.From)
	if err != nil {
		slog.Error("bot events upsert failed", "err", err)
		return
	}

	// Prefer the user's city when they have one set.
	filters := event.ListFilters{Limit: browseLimit}
	page, err := b.events.ListPublic(ctx, filters)
	if err != nil {
		slog.Error("bot list events failed", "err", err)
		b.send(ctx, msg.Chat.ID, "Something went wrong, please try again.", nil)
		return
	}
	_ = u

	if len(page.Items) == 0 {
		b.send(ctx, msg.Chat.ID,
			"No upcoming events yet. Check back soon or explore "+b.webBaseURL+"/events", nil)
		return
	}

	rows := make([][]models.InlineKeyboardButton, 0, len(page.Items))
	var sb strings.Builder
	sb.WriteString("📅 <b>Upcoming events</b>\n\n")
	for i, e := range page.Items {
		sb.WriteString(fmt.Sprintf("%d. <b>%s</b>\n    %s · %s\n",
			i+1, escape(e.Title), b.formatTime(e.StartsAt), escape(b.placeLabel(e))))
		rows = append(rows, []models.InlineKeyboardButton{{
			Text:         fmt.Sprintf("%d. %s", i+1, truncate(e.Title, 28)),
			CallbackData: "ev:" + strconv.FormatInt(e.ID, 10),
		}})
	}
	b.send(ctx, msg.Chat.ID, sb.String(),
		&models.InlineKeyboardMarkup{InlineKeyboard: rows})
}

func (b *Bot) handleEventDetail(ctx context.Context, _ *bot.Bot, update *models.Update) {
	cq := update.CallbackQuery
	if cq == nil {
		return
	}
	b.answerCallback(ctx, cq.ID, "")

	id, err := strconv.ParseInt(strings.TrimPrefix(cq.Data, "ev:"), 10, 64)
	if err != nil {
		return
	}
	e, err := b.events.GetPublished(ctx, id)
	if err != nil {
		b.send(ctx, chatIDOf(cq), "This event is no longer available.", nil)
		return
	}

	var sb strings.Builder
	sb.WriteString("🎟️ <b>" + escape(e.Title) + "</b>\n\n")
	sb.WriteString("🕐 " + b.formatTime(e.StartsAt) + "\n")
	sb.WriteString("📍 " + escape(b.placeLabel(e)) + "\n")
	sb.WriteString("👤 " + escape(e.OrganizerName) + "\n")
	sb.WriteString(fmt.Sprintf("👥 %d going", e.GoingCount))
	if e.Capacity != nil {
		sb.WriteString(fmt.Sprintf(" / %d spots", *e.Capacity))
	}
	if e.Description != "" {
		sb.WriteString("\n\n" + escape(truncate(e.Description, 300)))
	}

	markup := &models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{
		{{Text: "✅ Join event", CallbackData: "join:" + strconv.FormatInt(e.ID, 10)}},
		{{Text: "🌐 Open on Meetus.uz", URL: fmt.Sprintf("%s/events/%d", b.webBaseURL, e.ID)}},
	}}
	b.send(ctx, chatIDOf(cq), sb.String(), markup)
}

func (b *Bot) handleJoin(ctx context.Context, _ *bot.Bot, update *models.Update) {
	cq := update.CallbackQuery
	if cq == nil {
		return
	}
	id, err := strconv.ParseInt(strings.TrimPrefix(cq.Data, "join:"), 10, 64)
	if err != nil {
		return
	}
	u, err := b.upsertUser(ctx, &cq.From)
	if err != nil {
		b.answerCallback(ctx, cq.ID, "Something went wrong.")
		return
	}

	_, err = b.rsvps.Join(ctx, id, u.ID)
	if err != nil {
		// Service errors carry user-friendly messages ("event is full", ...).
		b.answerCallback(ctx, cq.ID, friendlyError(err))
		return
	}
	b.answerCallback(ctx, cq.ID, "You're in! 🎉")
	b.send(ctx, chatIDOf(cq),
		fmt.Sprintf("✅ You joined! Your QR ticket is ready:\n%s/tickets\n\n"+
			"I'll remind you before the event starts.", b.webBaseURL), nil)
}

func (b *Bot) answerCallback(ctx context.Context, id, text string) {
	_, err := b.api.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: id,
		Text:            text,
		ShowAlert:       text != "",
	})
	if err != nil {
		slog.Error("answer callback failed", "err", err)
	}
}

// SendReminder delivers one reminder message; used by the worker.
func (b *Bot) SendReminder(ctx context.Context, rem *notification.Reminder) error {
	lead := "starts in about an hour"
	if rem.Kind == notification.KindReminder24h {
		lead = "is coming up"
	}
	place := "Online"
	if !rem.IsOnline {
		parts := []string{}
		if rem.LocationName != nil {
			parts = append(parts, *rem.LocationName)
		}
		if rem.CitySlug != nil {
			parts = append(parts, *rem.CitySlug)
		}
		if len(parts) > 0 {
			place = strings.Join(parts, ", ")
		} else {
			place = "see event page"
		}
	}
	text := fmt.Sprintf("⏰ <b>%s</b> %s!\n\n🕐 %s\n📍 %s\n\n🎫 Your ticket: %s/tickets",
		escape(rem.EventTitle), lead, b.formatTime(rem.StartsAt), escape(place), b.webBaseURL)

	_, err := b.api.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    rem.UserTelegramID,
		Text:      text,
		ParseMode: models.ParseModeHTML,
	})
	return err
}

func (b *Bot) formatTime(t time.Time) string {
	return t.In(b.loc).Format("Mon, 2 Jan 15:04")
}

func (b *Bot) placeLabel(e *event.Event) string {
	if e.IsOnline {
		return "Online"
	}
	if e.LocationName != nil && *e.LocationName != "" {
		return *e.LocationName
	}
	if e.CitySlug != nil {
		return *e.CitySlug
	}
	return "In person"
}

func chatIDOf(cq *models.CallbackQuery) int64 {
	if cq.Message.Message != nil {
		return cq.Message.Message.Chat.ID
	}
	return cq.From.ID
}

func friendlyError(err error) string {
	s := err.Error()
	// apperr messages look like "conflict: this event is full" — show the
	// human part only.
	if _, after, ok := strings.Cut(s, ": "); ok {
		return after
	}
	return "Could not join this event."
}

func escape(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")
	return r.Replace(s)
}

func truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n-1]) + "…"
}
