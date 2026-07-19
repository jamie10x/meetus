// Package channel links a Telegram channel to an organizer profile, so
// the organizer can push event announcements to it. A channel is
// connected by adding the bot as an admin there (see tgbot's
// my_chat_member handler) — never by typing in an unverified chat ID —
// so a connection is proof the bot can actually post there.
package channel

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"meetus.uz/backend/internal/event"
	"meetus.uz/backend/internal/organizer"
	"meetus.uz/backend/internal/platform/apperr"
	"meetus.uz/backend/internal/platform/authn"
	"meetus.uz/backend/internal/platform/httpx"
	"meetus.uz/backend/internal/user"
)

// Announcer sends a formatted event announcement to a Telegram chat.
// Satisfied by *tgbot.Announcer. Defined here rather than imported, so
// this package doesn't depend on tgbot — tgbot already depends on
// channel (for the my_chat_member handler), and Go doesn't allow the
// reverse.
type Announcer interface {
	SendAnnouncement(ctx context.Context, chatID int64, langCode string, e *event.Event) error
}

type Channel struct {
	ID          int64
	OrganizerID int64
	ChatID      int64
	ChatTitle   string
	ConnectedAt time.Time
}

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// ConnectByTelegramID links a channel to the organizer owned by the given
// Telegram user, if one exists. Returns ok=false (no error) when that user
// has no organizer profile — the bot DMs a different message in that case
// rather than failing loudly.
func (r *Repository) ConnectByTelegramID(ctx context.Context, telegramID, chatID int64, chatTitle string) (organizerName string, ok bool, err error) {
	err = r.pool.QueryRow(ctx, `
		INSERT INTO channel_connections (organizer_id, chat_id, chat_title)
		SELECT o.id, $2, $3
		FROM organizers o
		JOIN users u ON u.id = o.user_id
		WHERE u.telegram_id = $1
		ON CONFLICT (chat_id) DO UPDATE SET
			organizer_id = EXCLUDED.organizer_id,
			chat_title   = EXCLUDED.chat_title,
			connected_at = now()
		RETURNING (SELECT display_name FROM organizers WHERE id = organizer_id)`,
		telegramID, chatID, chatTitle).Scan(&organizerName)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("connect channel: %w", err)
	}
	return organizerName, true, nil
}

// Disconnect removes a channel connection, e.g. when the bot is demoted
// or removed from the channel. Not an error if no row exists.
func (r *Repository) Disconnect(ctx context.Context, chatID int64) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM channel_connections WHERE chat_id = $1`, chatID)
	if err != nil {
		return fmt.Errorf("disconnect channel: %w", err)
	}
	return nil
}

func (r *Repository) ListForOrganizer(ctx context.Context, organizerID int64) ([]*Channel, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, organizer_id, chat_id, chat_title, connected_at
		FROM channel_connections WHERE organizer_id = $1
		ORDER BY connected_at DESC`, organizerID)
	if err != nil {
		return nil, fmt.Errorf("list channels: %w", err)
	}
	defer rows.Close()

	channels := make([]*Channel, 0, 4)
	for rows.Next() {
		var ch Channel
		if err := rows.Scan(&ch.ID, &ch.OrganizerID, &ch.ChatID, &ch.ChatTitle, &ch.ConnectedAt); err != nil {
			return nil, err
		}
		channels = append(channels, &ch)
	}
	return channels, rows.Err()
}

// GetOwned loads a channel connection and verifies it belongs to the
// given organizer, for use before sending an announcement or disconnecting.
func (r *Repository) GetOwned(ctx context.Context, organizerID, channelID int64) (*Channel, error) {
	var ch Channel
	err := r.pool.QueryRow(ctx, `
		SELECT id, organizer_id, chat_id, chat_title, connected_at
		FROM channel_connections WHERE id = $1`, channelID).
		Scan(&ch.ID, &ch.OrganizerID, &ch.ChatID, &ch.ChatTitle, &ch.ConnectedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperr.NotFound("channel not found")
	}
	if err != nil {
		return nil, fmt.Errorf("get channel: %w", err)
	}
	if ch.OrganizerID != organizerID {
		return nil, apperr.Forbidden("you do not own this channel")
	}
	return &ch, nil
}

type DTO struct {
	ID          int64     `json:"id"`
	ChatTitle   string    `json:"chatTitle"`
	ConnectedAt time.Time `json:"connectedAt"`
}

func (ch *Channel) ToDTO() DTO {
	return DTO{ID: ch.ID, ChatTitle: ch.ChatTitle, ConnectedAt: ch.ConnectedAt}
}

type Handler struct {
	repo      *Repository
	eventRepo *event.Repository
	users     *user.Repository
	announcer Announcer
}

func NewHandler(repo *Repository, eventRepo *event.Repository, users *user.Repository, announcer Announcer) *Handler {
	return &Handler{repo: repo, eventRepo: eventRepo, users: users, announcer: announcer}
}

func (h *Handler) Register(r gin.IRouter, requireAuth, requireOrganizer gin.HandlerFunc) {
	g := r.Group("/organizers/me/channels", requireAuth, requireOrganizer)
	g.GET("", h.list)
	g.DELETE("/:id", h.disconnect)

	r.POST("/events/:id/announce", requireAuth, requireOrganizer, h.announce)
}

func (h *Handler) list(c *gin.Context) {
	channels, err := h.repo.ListForOrganizer(c.Request.Context(), organizer.OrganizerID(c))
	if err != nil {
		httpx.Error(c, err)
		return
	}
	dtos := make([]DTO, len(channels))
	for i, ch := range channels {
		dtos[i] = ch.ToDTO()
	}
	httpx.OK(c, http.StatusOK, dtos)
}

func (h *Handler) disconnect(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		httpx.Error(c, apperr.Validation("invalid channel id"))
		return
	}
	ch, err := h.repo.GetOwned(c.Request.Context(), organizer.OrganizerID(c), id)
	if err != nil {
		httpx.Error(c, err)
		return
	}
	if err := h.repo.Disconnect(c.Request.Context(), ch.ChatID); err != nil {
		httpx.Error(c, err)
		return
	}
	httpx.OK(c, http.StatusOK, gin.H{"disconnected": true})
}

type announceRequest struct {
	ChannelID int64 `json:"channelId" binding:"required"`
}

// announce posts a published event to one of the caller's connected
// channels, in the caller's own language.
func (h *Handler) announce(c *gin.Context) {
	if h.announcer == nil {
		httpx.Error(c, apperr.Validation("channel announcements are not configured on this server"))
		return
	}

	eventID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || eventID <= 0 {
		httpx.Error(c, apperr.Validation("invalid event id"))
		return
	}
	var req announceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, apperr.Validation("channelId is required"))
		return
	}

	organizerID := organizer.OrganizerID(c)
	ctx := c.Request.Context()

	e, err := h.eventRepo.GetByID(ctx, eventID)
	if err != nil {
		httpx.Error(c, err)
		return
	}
	if e.OrganizerID != organizerID {
		httpx.Error(c, apperr.Forbidden("you do not own this event"))
		return
	}
	if e.Status != event.StatusPublished {
		httpx.Error(c, apperr.Conflict("only published events can be announced"))
		return
	}

	ch, err := h.repo.GetOwned(ctx, organizerID, req.ChannelID)
	if err != nil {
		httpx.Error(c, err)
		return
	}

	caller, err := h.users.GetByID(ctx, authn.UserID(c))
	if err != nil {
		httpx.Error(c, err)
		return
	}

	if err := h.announcer.SendAnnouncement(ctx, ch.ChatID, caller.Language, e); err != nil {
		httpx.Error(c, apperr.Wrap(apperr.CodeInternal,
			"could not send the announcement — check the bot still has admin rights in that channel", err))
		return
	}
	httpx.OK(c, http.StatusOK, gin.H{"sent": true})
}
