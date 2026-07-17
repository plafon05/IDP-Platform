package database

import (
	"context"
	"fmt"
	"time"

	"idp-platform/backend/internal/config"

	"github.com/jackc/pgx/v5/pgxpool"
)

const pingTimeout = 3 * time.Second

func Connect(ctx context.Context, cfg config.Config) (*pgxpool.Pool, error) {
	poolConfig, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database config: %w", err)
	}
	poolConfig.ConnConfig.RuntimeParams["statement_timeout"] = "30s"

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("create database pool: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, pingTimeout)
	defer cancel()

	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return pool, nil
}
