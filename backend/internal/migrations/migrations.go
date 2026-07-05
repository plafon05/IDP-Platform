package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"idp-platform/backend/internal/config"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

const migrationsDir = "migrations"

func Up(ctx context.Context, cfg config.Config) error {
	dbConfig, err := pgx.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("parse migration database config: %w", err)
	}

	db := stdlib.OpenDB(*dbConfig)
	defer db.Close()

	if err := ping(ctx, db); err != nil {
		return err
	}

	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("set migration dialect: %w", err)
	}

	if err := goose.UpContext(ctx, db, migrationsDir); err != nil {
		return fmt.Errorf("apply database migrations: %w", err)
	}

	return nil
}

func ping(ctx context.Context, db *sql.DB) error {
	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("ping migration database: %w", err)
	}

	return nil
}
