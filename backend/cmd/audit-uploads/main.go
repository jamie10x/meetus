// Command audit-uploads reports managed image usage and old, unreferenced
// candidates. It never deletes files. Run against a read-only volume mount.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"meetus.uz/backend/internal/config"
	"meetus.uz/backend/internal/platform/db"
	"meetus.uz/backend/internal/upload"
	"os"
	"time"
)

func run() error {
	age := flag.Duration("older-than", 30*24*time.Hour, "minimum candidate age (e.g. 720h)")
	details := flag.Int("details", 0, "maximum candidate filenames to include, 0-1000; default summary only")
	flag.Parse()
	if *age < 0 || *details < 0 || *details > 1000 {
		return fmt.Errorf("invalid age or detail limit")
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	urls, err := upload.NewRepository(pool).ReferencedURLs(ctx)
	if err != nil {
		return err
	}
	report, err := upload.Audit(ctx, cfg.UploadDir, urls, time.Now().Add(-*age), *details)
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(report)
}
func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "upload audit failed:", err)
		os.Exit(1)
	}
}
