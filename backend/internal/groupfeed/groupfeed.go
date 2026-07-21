// Package groupfeed tracks Telegram groups that have opted into the same
// platform-wide event feed as the official channel (see channel.Announcer
// and the official-channel config in router.go). Unlike channel
// connections, a group subscription isn't owned by any organizer — it's
// just "this group wants every new event", so it lives in its own small
// table rather than channel_connections.
package groupfeed

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// Subscribe adds (or refreshes the title of) a group's subscription.
func (r *Repository) Subscribe(ctx context.Context, chatID int64, chatTitle string) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO group_subscriptions (chat_id, chat_title)
		VALUES ($1, $2)
		ON CONFLICT (chat_id) DO UPDATE SET chat_title = EXCLUDED.chat_title`,
		chatID, chatTitle)
	if err != nil {
		return fmt.Errorf("subscribe group: %w", err)
	}
	return nil
}

// Unsubscribe removes a group, e.g. when the bot is kicked or leaves.
func (r *Repository) Unsubscribe(ctx context.Context, chatID int64) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM group_subscriptions WHERE chat_id = $1`, chatID)
	if err != nil {
		return fmt.Errorf("unsubscribe group: %w", err)
	}
	return nil
}

// ListChatIDs returns every subscribed group's chat ID, for fanning out
// an announcement.
func (r *Repository) ListChatIDs(ctx context.Context) ([]int64, error) {
	rows, err := r.pool.Query(ctx, `SELECT chat_id FROM group_subscriptions ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("list group subscriptions: %w", err)
	}
	defer rows.Close()

	ids := make([]int64, 0, 8)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
