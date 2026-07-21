// Package tgbot implements the Meetus Telegram bot: browsing events,
// one-tap RSVP, reminder delivery, and post-event feedback. Users are
// identified by their Telegram ID — the same identity used for website
// login. Messages are rendered in the user's language (see i18n.go).
package tgbot

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/redis/go-redis/v9"
	qrcode "github.com/skip2/go-qrcode"

	"meetus.uz/backend/internal/channel"
	"meetus.uz/backend/internal/event"
	"meetus.uz/backend/internal/feedback"
	"meetus.uz/backend/internal/groupfeed"
	"meetus.uz/backend/internal/notification"
	"meetus.uz/backend/internal/rsvp"
	"meetus.uz/backend/internal/user"
)

const browseLimit = 6

// feedbackCommentTTL bounds how long the bot waits for a follow-up
// comment after a star rating before giving up and treating any later
// message as ordinary chat again.
const feedbackCommentTTL = 10 * time.Minute

// weekday abbreviations per language, for date formatting. Month names
// are deliberately left numeric (DD.MM) — a common format in Uzbekistan
// and Russia — to avoid needing a second locale table.
var weekdayNames = map[lang][7]string{
	langEn: {"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"},
	langRu: {"Вс", "Пн", "Вт", "Ср", "Чт", "Пт", "Сб"},
	langUz: {"Yak", "Dush", "Sesh", "Chor", "Pay", "Jum", "Shan"},
}

type Bot struct {
	api        *bot.Bot
	users      *user.Repository
	events     *event.Repository
	rsvps      *rsvp.Repository
	feedback   *feedback.Repository
	channels   *channel.Repository
	groups     *groupfeed.Repository
	signer     *rsvp.TicketSigner
	redis      *redis.Client
	webBaseURL string
	loc        *time.Location
}

// Deps bundles Bot's constructor dependencies — plain positional params
// got unwieldy once ticket QR generation and group-feed support joined
// the original set.
type Deps struct {
	Token        string
	Users        *user.Repository
	Events       *event.Repository
	RSVPs        *rsvp.Repository
	Feedback     *feedback.Repository
	Channels     *channel.Repository
	Groups       *groupfeed.Repository
	TicketSigner *rsvp.TicketSigner
	Redis        *redis.Client
	WebBaseURL   string
}

func New(deps Deps) (*Bot, error) {
	b := &Bot{
		users:      deps.Users,
		events:     deps.Events,
		rsvps:      deps.RSVPs,
		feedback:   deps.Feedback,
		channels:   deps.Channels,
		groups:     deps.Groups,
		signer:     deps.TicketSigner,
		redis:      deps.Redis,
		webBaseURL: deps.WebBaseURL,
	}
	b.loc = tashkentLocation()

	api, err := bot.New(deps.Token, bot.WithDefaultHandler(b.handleDefault))
	if err != nil {
		return nil, fmt.Errorf("create telegram bot: %w", err)
	}
	b.api = api

	api.RegisterHandler(bot.HandlerTypeMessageText, "/start", bot.MatchTypePrefix, b.handleStart)
	api.RegisterHandler(bot.HandlerTypeMessageText, "/events", bot.MatchTypePrefix, b.handleEvents)
	api.RegisterHandler(bot.HandlerTypeMessageText, "/tickets", bot.MatchTypePrefix, b.handleTicketsCommand)
	api.RegisterHandler(bot.HandlerTypeMessageText, "/language", bot.MatchTypePrefix, b.handleLanguageCommand)
	api.RegisterHandler(bot.HandlerTypeMessageText, "/mute", bot.MatchTypePrefix, b.handleMuteCommand)
	api.RegisterHandler(bot.HandlerTypeMessageText, "/unmute", bot.MatchTypePrefix, b.handleUnmuteCommand)
	api.RegisterHandler(bot.HandlerTypeMessageText, "/digest", bot.MatchTypePrefix, b.handleDigestCommand)
	api.RegisterHandler(bot.HandlerTypeMessageText, "/nearby", bot.MatchTypePrefix, b.handleNearbyCommand)
	api.RegisterHandler(bot.HandlerTypeCallbackQueryData, "ev:", bot.MatchTypePrefix, b.handleEventDetail)
	api.RegisterHandler(bot.HandlerTypeCallbackQueryData, "join:", bot.MatchTypePrefix, b.handleJoin)
	api.RegisterHandler(bot.HandlerTypeCallbackQueryData, "lang:", bot.MatchTypePrefix, b.handleLanguageCallback)
	api.RegisterHandler(bot.HandlerTypeCallbackQueryData, "fb:", bot.MatchTypePrefix, b.handleFeedback)
	api.RegisterHandler(bot.HandlerTypeCallbackQueryData, "fbskip:", bot.MatchTypePrefix, b.handleFeedbackSkip)
	return b, nil
}

// Start runs long polling until ctx is canceled.
func (b *Bot) Start(ctx context.Context) {
	slog.Info("telegram bot polling started")
	b.api.Start(ctx)
}

// upsertUser registers or refreshes the Telegram user and returns the
// internal user record. A brand-new user's language is guessed from
// Telegram's own language_code; an existing user's choice is untouched
// (see user.TelegramProfile.Language).
func (b *Bot) upsertUser(ctx context.Context, from *models.User) (*user.User, error) {
	name := from.FirstName
	if from.LastName != "" {
		name += " " + from.LastName
	}
	return b.users.UpsertTelegramUser(ctx, user.TelegramProfile{
		TelegramID: from.ID,
		Name:       name,
		Username:   from.Username,
		Language:   mapTelegramLangCode(from.LanguageCode),
	})
}

// webURL builds a locale-correct site link (path must start with "/", or
// be "" for the home page) so bot-shared links land on the same language
// the bot is already speaking, instead of round-tripping through the
// browser's own locale detection.
func (b *Bot) webURL(l lang, path string) string {
	return buildWebURL(b.webBaseURL, l, path)
}

// tashkentLocation is shared by Bot and Announcer for date formatting —
// there is exactly one timezone the whole product cares about.
func tashkentLocation() *time.Location {
	loc, err := time.LoadLocation("Asia/Tashkent")
	if err != nil {
		return time.FixedZone("UZT", 5*3600)
	}
	return loc
}

func buildWebURL(webBaseURL string, l lang, path string) string {
	return webBaseURL + "/" + string(l) + path
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
	u, err := b.upsertUser(ctx, msg.From)
	if err != nil {
		slog.Error("bot start upsert failed", "err", err)
		return
	}
	l := normalizeLang(u.Language)
	b.send(ctx, msg.Chat.ID, tf(l, kWelcome, msg.From.FirstName, b.webURL(l, "")), nil)
}

func (b *Bot) handleDefault(ctx context.Context, _ *bot.Bot, update *models.Update) {
	// No dedicated registration mechanism for non-message/callback update
	// types in this library — my_chat_member updates land here too.
	if update.MyChatMember != nil {
		b.handleMyChatMember(ctx, update.MyChatMember)
		return
	}
	if update.Message == nil || update.Message.From == nil {
		return
	}
	u, err := b.upsertUser(ctx, update.Message.From)
	l := langEn
	if err == nil {
		l = normalizeLang(u.Language)
	}

	if update.Message.Location != nil {
		b.handleLocationShared(ctx, update.Message, l)
		return
	}

	text := update.Message.Text
	if b.redis != nil && text != "" && !strings.HasPrefix(text, "/") {
		if eventID, ok := b.popPendingFeedbackComment(ctx, update.Message.From.ID); ok {
			if err := b.feedback.SetComment(ctx, eventID, u.ID, text); err != nil {
				slog.Error("set feedback comment failed", "event_id", eventID, "err", err)
			}
			b.send(ctx, update.Message.Chat.ID, t(l, kFeedbackCommentThanks), nil)
			return
		}
	}

	b.send(ctx, update.Message.Chat.ID, tf(l, kDefaultHint, b.webURL(l, "")), nil)
}

// feedbackAwaitKey scopes the pending-comment marker to one Telegram user;
// eventID is stored as the value so the later free-text reply knows which
// event's feedback row to attach the comment to.
func feedbackAwaitKey(telegramID int64) string {
	return fmt.Sprintf("meetus:feedback-comment-await:%d", telegramID)
}

func (b *Bot) awaitFeedbackComment(ctx context.Context, telegramID, eventID int64) {
	if err := b.redis.Set(ctx, feedbackAwaitKey(telegramID), eventID, feedbackCommentTTL).Err(); err != nil {
		slog.Error("set pending feedback comment failed", "err", err)
	}
}

// popPendingFeedbackComment atomically reads and clears the pending marker,
// so a message can only ever be consumed as one event's comment.
func (b *Bot) popPendingFeedbackComment(ctx context.Context, telegramID int64) (eventID int64, ok bool) {
	val, err := b.redis.GetDel(ctx, feedbackAwaitKey(telegramID)).Result()
	if err != nil {
		return 0, false
	}
	eventID, err = strconv.ParseInt(val, 10, 64)
	if err != nil {
		return 0, false
	}
	return eventID, true
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
	l := normalizeLang(u.Language)

	// Prefer the user's city when they have one set.
	filters := event.ListFilters{Limit: browseLimit}
	page, err := b.events.ListPublic(ctx, filters)
	if err != nil {
		slog.Error("bot list events failed", "err", err)
		b.send(ctx, msg.Chat.ID, t(l, kErrGeneric), nil)
		return
	}

	if len(page.Items) == 0 {
		b.send(ctx, msg.Chat.ID, tf(l, kNoEvents, b.webURL(l, "")), nil)
		return
	}

	rows := make([][]models.InlineKeyboardButton, 0, len(page.Items))
	var sb strings.Builder
	sb.WriteString(t(l, kEventsHeader))
	for i, e := range page.Items {
		sb.WriteString(fmt.Sprintf("%d. <b>%s</b>\n    %s · %s\n",
			i+1, escape(e.Title), b.formatTime(l, e.StartsAt), escape(b.placeLabel(l, e))))
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

	u, err := b.upsertUser(ctx, &cq.From)
	l := langEn
	if err == nil {
		l = normalizeLang(u.Language)
	}

	id, err := strconv.ParseInt(strings.TrimPrefix(cq.Data, "ev:"), 10, 64)
	if err != nil {
		return
	}
	e, err := b.events.GetPublished(ctx, id)
	if err != nil {
		b.send(ctx, chatIDOf(cq), t(l, kEventUnavailable), nil)
		return
	}

	var sb strings.Builder
	sb.WriteString("🎟️ <b>" + escape(e.Title) + "</b>\n\n")
	sb.WriteString("🕐 " + b.formatTime(l, e.StartsAt) + "\n")
	sb.WriteString("📍 " + escape(b.placeLabel(l, e)) + "\n")
	sb.WriteString("👤 " + escape(e.OrganizerName) + "\n")
	sb.WriteString("👥 " + tf(l, kGoingCount, e.GoingCount))
	if e.Capacity != nil {
		sb.WriteString(tf(l, kSpotsLeft, *e.Capacity))
	}
	if e.Description != "" {
		sb.WriteString("\n\n" + escape(truncate(e.Description, 300)))
	}

	// WebApp (not URL) keeps the user inside Telegram: the event page opens
	// as a Mini App in place instead of switching to an external browser.
	// Telegram only allows this button type in private-chat messages,
	// which is the bot's only context.
	markup := &models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{
		{{Text: t(l, kJoinButton), CallbackData: "join:" + strconv.FormatInt(e.ID, 10)}},
		{{Text: t(l, kOpenWebButton), WebApp: &models.WebAppInfo{
			URL: b.webURL(l, fmt.Sprintf("/events/%d", e.ID)),
		}}},
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
		b.answerCallback(ctx, cq.ID, t(langEn, kErrGeneric))
		return
	}
	l := normalizeLang(u.Language)

	res, err := b.rsvps.Join(ctx, id, u.ID)
	if err != nil {
		b.answerCallback(ctx, cq.ID, friendlyError(l, err))
		return
	}

	if res.Status == "waitlisted" {
		b.answerCallback(ctx, cq.ID, t(l, kJoinedAlert))
		b.send(ctx, chatIDOf(cq), t(l, kWaitlisted), nil)
		return
	}

	b.answerCallback(ctx, cq.ID, t(l, kJoinedAlert))
	b.send(ctx, chatIDOf(cq), t(l, kJoinedSuccess), nil)

	e, err := b.events.GetPublished(ctx, id)
	if err != nil {
		slog.Error("bot join: could not load event for ticket caption", "event_id", id, "err", err)
		return
	}
	b.sendTicketPhoto(ctx, chatIDOf(cq), l, res.Ticket.Code, e)
}

// sendTicketPhoto generates the QR code for a ticket and sends it as a
// photo message — the point of this being a photo rather than a link is
// that it works without leaving the chat or loading the website at all.
func (b *Bot) sendTicketPhoto(ctx context.Context, chatID int64, l lang, code string, e *event.Event) {
	if err := sendTicketPhotoTo(ctx, b.api, b.loc, b.signer, chatID, l, code, e); err != nil {
		slog.Error("send ticket photo failed", "chat_id", chatID, "err", err)
	}
}

// sendTicketPhotoTo renders a ticket's QR code and sends it as a photo
// message to chatID. A package-level function (not a *Bot method) since
// Announcer needs the exact same rendering for waitlist-promotion
// tickets and has no *Bot to call into — see the shared-helper pattern
// note in AGENTS.md.
func sendTicketPhotoTo(ctx context.Context, api *bot.Bot, loc *time.Location, signer *rsvp.TicketSigner, chatID int64, l lang, code string, e *event.Event) error {
	png, err := qrcode.Encode(signer.QRValue(code), qrcode.Medium, 512)
	if err != nil {
		return fmt.Errorf("qr encode: %w", err)
	}
	caption := tf(l, kTicketCaption, escape(e.Title), formatEventTime(l, e.StartsAt, loc), escape(eventPlaceLabel(l, e)))
	_, err = api.SendPhoto(ctx, &bot.SendPhotoParams{
		ChatID:    chatID,
		Photo:     &models.InputFileUpload{Filename: "ticket.png", Data: bytes.NewReader(png)},
		Caption:   caption,
		ParseMode: models.ParseModeHTML,
	})
	return err
}

// handleTicketsCommand resends the QR photo for every upcoming ticket —
// how someone retrieves their ticket again without digging through chat
// history or opening the website.
func (b *Bot) handleTicketsCommand(ctx context.Context, _ *bot.Bot, update *models.Update) {
	msg := update.Message
	if msg == nil || msg.From == nil {
		return
	}
	u, err := b.upsertUser(ctx, msg.From)
	if err != nil {
		slog.Error("bot tickets upsert failed", "err", err)
		return
	}
	l := normalizeLang(u.Language)

	tickets, err := b.rsvps.ListMyTickets(ctx, u.ID)
	if err != nil {
		slog.Error("bot tickets list failed", "err", err)
		b.send(ctx, msg.Chat.ID, t(l, kErrGeneric), nil)
		return
	}

	const maxTicketsShown = 5
	sent := 0
	for _, mt := range tickets {
		if sent >= maxTicketsShown {
			break
		}
		if mt.EventStatus != "published" || !mt.StartsAt.After(time.Now()) {
			continue
		}
		e, err := b.events.GetPublished(ctx, mt.EventID)
		if err != nil {
			continue
		}
		b.sendTicketPhoto(ctx, msg.Chat.ID, l, mt.Code, e)
		sent++
	}
	if sent == 0 {
		b.send(ctx, msg.Chat.ID, t(l, kNoUpcomingTickets), nil)
	}
}

func (b *Bot) handleMuteCommand(ctx context.Context, _ *bot.Bot, update *models.Update) {
	msg := update.Message
	if msg == nil || msg.From == nil {
		return
	}
	u, err := b.upsertUser(ctx, msg.From)
	if err != nil {
		slog.Error("bot mute upsert failed", "err", err)
		return
	}
	l := normalizeLang(u.Language)
	if err := b.users.SetNotificationsMuted(ctx, u.ID, true); err != nil {
		slog.Error("bot mute failed", "err", err)
		return
	}
	b.send(ctx, msg.Chat.ID, t(l, kMuted), nil)
}

func (b *Bot) handleUnmuteCommand(ctx context.Context, _ *bot.Bot, update *models.Update) {
	msg := update.Message
	if msg == nil || msg.From == nil {
		return
	}
	u, err := b.upsertUser(ctx, msg.From)
	if err != nil {
		slog.Error("bot unmute upsert failed", "err", err)
		return
	}
	l := normalizeLang(u.Language)
	if err := b.users.SetNotificationsMuted(ctx, u.ID, false); err != nil {
		slog.Error("bot unmute failed", "err", err)
		return
	}
	b.send(ctx, msg.Chat.ID, t(l, kUnmuted), nil)
}

// handleDigestCommand toggles the opt-in weekly digest — on if currently
// off, off if currently on, so the same /digest command works both ways.
func (b *Bot) handleDigestCommand(ctx context.Context, _ *bot.Bot, update *models.Update) {
	msg := update.Message
	if msg == nil || msg.From == nil {
		return
	}
	u, err := b.upsertUser(ctx, msg.From)
	if err != nil {
		slog.Error("bot digest upsert failed", "err", err)
		return
	}
	l := normalizeLang(u.Language)
	enable := !u.WeeklyDigestEnabled
	if err := b.users.SetWeeklyDigest(ctx, u.ID, enable); err != nil {
		slog.Error("bot digest toggle failed", "err", err)
		return
	}
	if enable {
		b.send(ctx, msg.Chat.ID, t(l, kDigestOn), nil)
	} else {
		b.send(ctx, msg.Chat.ID, t(l, kDigestOff), nil)
	}
}

// handleNearbyCommand asks for a location share via Telegram's native
// request-location keyboard button — a bot cannot request location any
// other way. The actual search happens in handleLocationShared once the
// share arrives.
func (b *Bot) handleNearbyCommand(ctx context.Context, _ *bot.Bot, update *models.Update) {
	msg := update.Message
	if msg == nil || msg.From == nil {
		return
	}
	u, err := b.upsertUser(ctx, msg.From)
	l := langEn
	if err == nil {
		l = normalizeLang(u.Language)
	}
	markup := &models.ReplyKeyboardMarkup{
		Keyboard: [][]models.KeyboardButton{
			{{Text: t(l, kShareLocationButton), RequestLocation: true}},
		},
		ResizeKeyboard:  true,
		OneTimeKeyboard: true,
	}
	b.send(ctx, msg.Chat.ID, t(l, kNearbyPrompt), markup)
}

// searchRadiusKm bounds the nearby-events search — a fixed radius keeps
// this simple; no UI exists yet to let someone adjust it.
const searchRadiusKm = 15

// handleLocationShared runs the nearby-events search once someone shares
// their location (via /nearby's keyboard button, or Telegram's own
// attach-location flow — both arrive as an ordinary message with
// Location set, so handleDefault routes either one here).
func (b *Bot) handleLocationShared(ctx context.Context, msg *models.Message, l lang) {
	loc := msg.Location
	nearby, err := b.events.ListNearby(ctx, loc.Latitude, loc.Longitude, searchRadiusKm, browseLimit)
	if err != nil {
		slog.Error("bot nearby search failed", "err", err)
		b.send(ctx, msg.Chat.ID, t(l, kErrGeneric), nil)
		return
	}
	if len(nearby) == 0 {
		b.send(ctx, msg.Chat.ID, t(l, kNearbyEmpty), nil)
		return
	}

	rows := make([][]models.InlineKeyboardButton, 0, len(nearby))
	var sb strings.Builder
	sb.WriteString(t(l, kNearbyHeader))
	for i, ne := range nearby {
		sb.WriteString(fmt.Sprintf("%d. <b>%s</b> · %.1f km\n    %s · %s\n",
			i+1, escape(ne.Title), ne.DistanceKm, b.formatTime(l, ne.StartsAt), escape(b.placeLabel(l, &ne.Event))))
		rows = append(rows, []models.InlineKeyboardButton{{
			Text:         fmt.Sprintf("%d. %s", i+1, truncate(ne.Title, 28)),
			CallbackData: "ev:" + strconv.FormatInt(ne.ID, 10),
		}})
	}
	b.send(ctx, msg.Chat.ID, sb.String(), &models.InlineKeyboardMarkup{InlineKeyboard: rows})
}

// handleLanguageCommand shows the language picker.
func (b *Bot) handleLanguageCommand(ctx context.Context, _ *bot.Bot, update *models.Update) {
	msg := update.Message
	if msg == nil || msg.From == nil {
		return
	}
	u, err := b.upsertUser(ctx, msg.From)
	l := langEn
	if err == nil {
		l = normalizeLang(u.Language)
	}
	b.send(ctx, msg.Chat.ID, t(l, kLanguagePrompt), languagePickerMarkup())
}

func languagePickerMarkup() *models.InlineKeyboardMarkup {
	return &models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{
		{{Text: langDisplayName(langUz), CallbackData: "lang:uz"}},
		{{Text: langDisplayName(langRu), CallbackData: "lang:ru"}},
		{{Text: langDisplayName(langEn), CallbackData: "lang:en"}},
	}}
}

func (b *Bot) handleLanguageCallback(ctx context.Context, _ *bot.Bot, update *models.Update) {
	cq := update.CallbackQuery
	if cq == nil {
		return
	}
	b.answerCallback(ctx, cq.ID, "")

	newLang := normalizeLang(strings.TrimPrefix(cq.Data, "lang:"))
	u, err := b.upsertUser(ctx, &cq.From)
	if err != nil {
		return
	}
	code := string(newLang)
	if _, err := b.users.UpdateProfile(ctx, u.ID, user.ProfileUpdate{Language: &code}); err != nil {
		slog.Error("bot language update failed", "err", err)
		return
	}
	b.send(ctx, chatIDOf(cq), tf(newLang, kLanguageSet, langDisplayName(newLang)), nil)
}

// handleMyChatMember fires whenever the bot's own membership changes in a
// chat. The only case handled: the bot is made an admin of a channel —
// that's the verified proof needed to link the channel to whoever added
// it, if they have an organizer profile. Any other transition (demoted,
// removed, left) disconnects the channel, since the bot can no longer
// post there.
func (b *Bot) handleMyChatMember(ctx context.Context, mcm *models.ChatMemberUpdated) {
	switch mcm.Chat.Type {
	case models.ChatTypeChannel:
		b.handleChannelMembership(ctx, mcm)
	case models.ChatTypeGroup, models.ChatTypeSupergroup:
		b.handleGroupMembership(ctx, mcm)
	}
}

func (b *Bot) handleChannelMembership(ctx context.Context, mcm *models.ChatMemberUpdated) {
	adderID := mcm.From.ID
	u, err := b.upsertUser(ctx, &mcm.From)
	l := langEn
	if err == nil {
		l = normalizeLang(u.Language)
	}

	switch mcm.NewChatMember.Type {
	case models.ChatMemberTypeAdministrator, models.ChatMemberTypeOwner:
		organizerName, ok, err := b.channels.ConnectByTelegramID(ctx, adderID, mcm.Chat.ID, mcm.Chat.Title)
		if err != nil {
			slog.Error("channel connect failed", "chat_id", mcm.Chat.ID, "err", err)
			return
		}
		if ok {
			slog.Info("channel connected", "chat_id", mcm.Chat.ID, "organizer", organizerName)
			b.send(ctx, adderID, tf(l, kChannelConnected, escape(mcm.Chat.Title)), nil)
		} else {
			// Best-effort DM; fails silently if they've never started the
			// bot (Telegram requires a prior interaction to message a user).
			b.send(ctx, adderID, t(l, kChannelConnectNeedsOrganizer), nil)
		}
	default:
		if err := b.channels.Disconnect(ctx, mcm.Chat.ID); err != nil {
			slog.Error("channel disconnect failed", "chat_id", mcm.Chat.ID, "err", err)
		}
	}
}

// handleGroupMembership subscribes/unsubscribes a group to the same
// platform-wide feed the official channel gets (see groupfeed package) —
// unlike channels, no organizer ownership or admin rights are required;
// any group the bot is a member of (and hasn't been kicked/left) is
// subscribed.
func (b *Bot) handleGroupMembership(ctx context.Context, mcm *models.ChatMemberUpdated) {
	if b.groups == nil {
		return
	}
	switch mcm.NewChatMember.Type {
	case models.ChatMemberTypeLeft, models.ChatMemberTypeBanned:
		if err := b.groups.Unsubscribe(ctx, mcm.Chat.ID); err != nil {
			slog.Error("group unsubscribe failed", "chat_id", mcm.Chat.ID, "err", err)
		}
	default:
		if err := b.groups.Subscribe(ctx, mcm.Chat.ID, mcm.Chat.Title); err != nil {
			slog.Error("group subscribe failed", "chat_id", mcm.Chat.ID, "err", err)
			return
		}
		slog.Info("group subscribed", "chat_id", mcm.Chat.ID, "title", mcm.Chat.Title)
		u, err := b.upsertUser(ctx, &mcm.From)
		l := langEn
		if err == nil {
			l = normalizeLang(u.Language)
		}
		b.send(ctx, mcm.Chat.ID, t(l, kGroupSubscribed), nil)
	}
}

// handleFeedback records a tapped star rating. Repeated taps upsert
// (Repository.Submit is idempotent), so no extra guard is needed here.
func (b *Bot) handleFeedback(ctx context.Context, _ *bot.Bot, update *models.Update) {
	cq := update.CallbackQuery
	if cq == nil {
		return
	}
	u, err := b.upsertUser(ctx, &cq.From)
	if err != nil {
		b.answerCallback(ctx, cq.ID, t(langEn, kErrGeneric))
		return
	}
	l := normalizeLang(u.Language)

	eventID, rating, ok := parseFeedbackCallback(cq.Data)
	if !ok {
		return
	}
	if err := b.feedback.Submit(ctx, eventID, u.ID, rating); err != nil {
		b.answerCallback(ctx, cq.ID, friendlyError(l, err))
		return
	}
	b.answerCallback(ctx, cq.ID, t(l, kFeedbackThanks))

	if b.redis == nil {
		return
	}
	b.awaitFeedbackComment(ctx, cq.From.ID, eventID)
	markup := &models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{
		{{Text: t(l, kSkipButton), CallbackData: "fbskip:" + strconv.FormatInt(eventID, 10)}},
	}}
	b.send(ctx, chatIDOf(cq), t(l, kFeedbackCommentPrompt), markup)
}

// handleFeedbackSkip cancels the pending comment prompt when the attendee
// taps Skip instead of typing a reply.
func (b *Bot) handleFeedbackSkip(ctx context.Context, _ *bot.Bot, update *models.Update) {
	cq := update.CallbackQuery
	if cq == nil {
		return
	}
	b.answerCallback(ctx, cq.ID, "")
	if b.redis != nil {
		b.redis.Del(ctx, feedbackAwaitKey(cq.From.ID))
	}
}

// parseFeedbackCallback parses "fb:<eventID>:<rating>".
func parseFeedbackCallback(data string) (eventID int64, rating int, ok bool) {
	rest, ratingStr, found := strings.Cut(strings.TrimPrefix(data, "fb:"), ":")
	if !found {
		return 0, 0, false
	}
	eventID, err := strconv.ParseInt(rest, 10, 64)
	if err != nil {
		return 0, 0, false
	}
	rating, err = strconv.Atoi(ratingStr)
	if err != nil {
		return 0, 0, false
	}
	return eventID, rating, true
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
	l := normalizeLang(rem.UserLanguage)
	key := kReminder1h
	if rem.Kind == notification.KindReminder24h {
		key = kReminder24h
	}
	place := t(l, kPlaceOnline)
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
			place = t(l, kPlaceSeeEventPage)
		}
	}
	text := tf(l, key, escape(rem.EventTitle), b.formatTime(l, rem.StartsAt), escape(place), b.webURL(l, "/tickets"))

	_, err := b.api.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    rem.UserTelegramID,
		Text:      text,
		ParseMode: models.ParseModeHTML,
	})
	return err
}

// SendFeedbackRequest prompts one attendee to rate a finished event.
func (b *Bot) SendFeedbackRequest(ctx context.Context, f *notification.FeedbackDue) error {
	l := normalizeLang(f.UserLanguage)
	text := tf(l, kFeedbackPrompt, escape(f.EventTitle))

	buttons := make([]models.InlineKeyboardButton, 5)
	for i := 1; i <= 5; i++ {
		buttons[i-1] = models.InlineKeyboardButton{
			Text:         strings.Repeat("⭐", i),
			CallbackData: fmt.Sprintf("fb:%d:%d", f.EventID, i),
		}
	}

	_, err := b.api.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      f.UserTelegramID,
		Text:        text,
		ParseMode:   models.ParseModeHTML,
		ReplyMarkup: &models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{buttons}},
	})
	return err
}

// SendWeeklyDigest sends one subscriber their "what's on this week"
// listing, scoped to their city if they've set one. Silently does
// nothing when there's nothing upcoming in the window — an empty digest
// isn't worth a notification.
func (b *Bot) SendWeeklyDigest(ctx context.Context, sub *user.DigestSubscriber) error {
	l := normalizeLang(sub.Language)
	until := time.Now().Add(7 * 24 * time.Hour)
	page, err := b.events.ListPublic(ctx, event.ListFilters{
		CityID: sub.CityID,
		To:     &until,
		Limit:  browseLimit,
	})
	if err != nil {
		return fmt.Errorf("weekly digest: list events: %w", err)
	}
	if len(page.Items) == 0 {
		return nil
	}

	rows := make([][]models.InlineKeyboardButton, 0, len(page.Items))
	var sb strings.Builder
	sb.WriteString(t(l, kDigestHeader))
	for i, e := range page.Items {
		sb.WriteString(fmt.Sprintf("%d. <b>%s</b>\n    %s · %s\n",
			i+1, escape(e.Title), formatEventTime(l, e.StartsAt, b.loc), escape(eventPlaceLabel(l, e))))
		rows = append(rows, []models.InlineKeyboardButton{{
			Text:         fmt.Sprintf("%d. %s", i+1, truncate(e.Title, 28)),
			CallbackData: "ev:" + strconv.FormatInt(e.ID, 10),
		}})
	}

	_, err = b.api.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      sub.TelegramID,
		Text:        sb.String(),
		ParseMode:   models.ParseModeHTML,
		ReplyMarkup: &models.InlineKeyboardMarkup{InlineKeyboard: rows},
	})
	return err
}

func (b *Bot) formatTime(l lang, tm time.Time) string {
	return formatEventTime(l, tm, b.loc)
}

func formatEventTime(l lang, tm time.Time, loc *time.Location) string {
	local := tm.In(loc)
	weekday := weekdayNames[l][int(local.Weekday())]
	return weekday + ", " + local.Format("02.01 15:04")
}

func (b *Bot) placeLabel(l lang, e *event.Event) string {
	return eventPlaceLabel(l, e)
}

func eventPlaceLabel(l lang, e *event.Event) string {
	if e.IsOnline {
		return t(l, kPlaceOnline)
	}
	if e.LocationName != nil && *e.LocationName != "" {
		return *e.LocationName
	}
	if e.CitySlug != nil {
		return *e.CitySlug
	}
	return t(l, kPlaceInPerson)
}

func chatIDOf(cq *models.CallbackQuery) int64 {
	if cq.Message.Message != nil {
		return cq.Message.Message.Chat.ID
	}
	return cq.From.ID
}

// friendlyError translates the small closed set of known service error
// messages (RSVP/feedback conflicts); anything else falls back to a
// generic translated message rather than leaking raw English internals.
func friendlyError(l lang, err error) string {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "already joined"):
		return t(l, kErrAlreadyJoined)
	case strings.Contains(msg, "event is full"):
		return t(l, kErrEventFull)
	case strings.Contains(msg, "not open for RSVPs"):
		return t(l, kErrNotOpen)
	case strings.Contains(msg, "already started"):
		return t(l, kErrAlreadyStarted)
	default:
		return t(l, kErrGeneric)
	}
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
