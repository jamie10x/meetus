package workerhealth

import (
	"context"
	"encoding/json"
	"github.com/redis/go-redis/v9"
	"os"
	"testing"
	"time"
)

func TestProgressExpiresAndRedisFailureIsNotHealthy(t *testing.T) {
	ctx := context.Background()
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}
	client := redis.NewClient(&redis.Options{Addr: addr})
	defer client.Close()
	if err := client.Ping(ctx).Err(); err != nil {
		if os.Getenv("CI") != "" {
			t.Fatal(err)
		}
		t.Skipf("redis unavailable: %v", err)
	}
	client.Del(ctx, Key)
	defer client.Del(ctx, Key)
	missing, err := Read(ctx, client, true)
	if err != nil || missing.Status != "stale" {
		t.Fatalf("missing: %+v %v", missing, err)
	}
	if err = New(client, "test-revision").Pulse(ctx); err != nil {
		t.Fatal(err)
	}
	live, err := Read(ctx, client, true)
	if err != nil || live.Status != "ready" || live.Revision == nil || *live.Revision != "test-revision" {
		t.Fatalf("live: %+v %v", live, err)
	}
	ttl, err := client.TTL(ctx, Key).Result()
	if err != nil || ttl <= 0 || ttl > TTL {
		t.Fatalf("heartbeat must expire: %v %v", ttl, err)
	}
	old := time.Now().Add(-2 * TTL)
	data, _ := json.Marshal(Status{Status: "ready", LastProgressAt: &old})
	if err = client.Set(ctx, Key, data, TTL).Err(); err != nil {
		t.Fatal(err)
	}
	stale, err := Read(ctx, client, true)
	if err != nil || stale.Status != "stale" {
		t.Fatalf("old progress: %+v %v", stale, err)
	}
	client.Close()
	if _, err = Read(ctx, client, true); err == nil {
		t.Fatal("redis outage was hidden")
	}
	disabled, err := Read(ctx, client, false)
	if err != nil || disabled.Status != "disabled" {
		t.Fatal("unconfigured worker should be explicit")
	}
}
