// Package workerhealth records successful delivery-loop progress, not merely
// process uptime. The API reads the same expiring key for operator visibility.
package workerhealth

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

const Key = "meetus:worker:delivery-progress"
const TTL = 90 * time.Second

type Status struct {
	Status         string     `json:"status"`
	LastProgressAt *time.Time `json:"lastProgressAt"`
	Revision       *string    `json:"revision"`
}

type Monitor struct {
	redis       *redis.Client
	revision    string
	lastAttempt time.Time // Used only by the single delivery-loop goroutine.
}

func New(rdb *redis.Client, revision string) *Monitor {
	return &Monitor{redis: rdb, revision: revision}
}
func (m *Monitor) Pulse(ctx context.Context) error {
	now := time.Now().UTC()
	if now.Sub(m.lastAttempt) < 15*time.Second {
		return nil
	}
	m.lastAttempt = now
	value, err := json.Marshal(Status{Status: "ready", LastProgressAt: &now, Revision: &m.revision})
	if err != nil {
		return err
	}
	writeCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err = m.redis.Set(writeCtx, Key, value, TTL).Err(); err != nil {
		return err
	}
	return nil
}
func Read(ctx context.Context, rdb *redis.Client, enabled bool) (Status, error) {
	if !enabled {
		return Status{Status: "disabled"}, nil
	}
	value, err := rdb.Get(ctx, Key).Bytes()
	if errors.Is(err, redis.Nil) {
		return Status{Status: "stale"}, nil
	}
	if err != nil {
		return Status{}, err
	}
	var result Status
	if err = json.Unmarshal(value, &result); err != nil {
		return Status{}, err
	}
	result.Status = "stale"
	if result.LastProgressAt != nil && time.Since(*result.LastProgressAt) >= 0 && time.Since(*result.LastProgressAt) < TTL {
		result.Status = "ready"
	}
	return result, nil
}
