// Command migrate applies database migrations embedded in the binary.
//
// Usage:
//
//	migrate up          apply all pending migrations
//	migrate down 1      roll back N migrations
//	migrate version     print current schema version
package main

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"

	migrationsfs "meetus.uz/backend/db"
	"meetus.uz/backend/internal/config"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		slog.Error("migrate failed", "err", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: migrate up | down <n> | version")
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	src, err := iofs.New(migrationsfs.Migrations, "migrations")
	if err != nil {
		return fmt.Errorf("load embedded migrations: %w", err)
	}

	m, err := migrate.NewWithSourceInstance("iofs", src, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer m.Close()

	switch args[0] {
	case "up":
		err = m.Up()
	case "down":
		if len(args) < 2 {
			return fmt.Errorf("usage: migrate down <n>")
		}
		var n int
		if n, err = strconv.Atoi(args[1]); err != nil {
			return fmt.Errorf("invalid step count %q", args[1])
		}
		err = m.Steps(-n)
	case "version":
		v, dirty, verr := m.Version()
		if verr != nil {
			return verr
		}
		fmt.Printf("version=%d dirty=%v\n", v, dirty)
		return nil
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}

	if errors.Is(err, migrate.ErrNoChange) {
		slog.Info("no pending migrations")
		return nil
	}
	if err != nil {
		return err
	}
	slog.Info("migrations applied")
	return nil
}
