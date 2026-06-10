package auth

import (
	"context"
	"errors"
	"log/slog"

	"idp-platform/backend/internal/config"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func SeedAdmin(ctx context.Context, cfg config.Config, db *pgxpool.Pool) error {
	if cfg.AppEnv == "production" {
		return nil
	}
	if cfg.SeedAdminEmail == "" || cfg.SeedAdminPassword == "" {
		return nil
	}

	var existingID string
	err := db.QueryRow(ctx, `
		SELECT id::text
		FROM users
		WHERE lower(email) = lower($1)
	`, cfg.SeedAdminEmail).Scan(&existingID)
	if err == nil {
		return nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}

	passwordHash, err := HashPassword(cfg.SeedAdminPassword)
	if err != nil {
		return err
	}

	tx, err := db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var userID string
	err = tx.QueryRow(ctx, `
		INSERT INTO users (email, password_hash, first_name, last_name, position)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id::text
	`, cfg.SeedAdminEmail, passwordHash, cfg.SeedAdminFirstName, cfg.SeedAdminLastName, "HR Admin").Scan(&userID)
	if err != nil {
		return err
	}

	for _, role := range []string{"employee", "hr_admin"} {
		if _, err := tx.Exec(ctx, `
			INSERT INTO user_roles (user_id, role)
			VALUES ($1, $2)
		`, userID, role); err != nil {
			return err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}

	slog.Info("seed admin user created", "email", cfg.SeedAdminEmail)
	return nil
}
