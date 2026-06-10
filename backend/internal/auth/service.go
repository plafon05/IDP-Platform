package auth

import (
	"context"
	"errors"
	"strings"
	"time"

	"idp-platform/backend/internal/config"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	maxFailedLoginAttempts = 5
	lockoutDuration        = 15 * time.Minute
)

var (
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrUserBlocked        = errors.New("user account is blocked")
	ErrUserLocked         = errors.New("user account is temporarily locked")
	ErrInvalidToken       = errors.New("invalid refresh token")
)

type Service struct {
	cfg config.Config
	db  *pgxpool.Pool
}

type User struct {
	ID         string   `json:"id"`
	Email      string   `json:"email"`
	FirstName  string   `json:"first_name"`
	LastName   string   `json:"last_name"`
	MiddleName *string  `json:"middle_name,omitempty"`
	Position   string   `json:"position"`
	Roles      []string `json:"roles"`
}

type TokenPair struct {
	AccessToken           string
	AccessTokenExpiresAt  time.Time
	RefreshToken          string
	RefreshTokenExpiresAt time.Time
	User                  User
}

func NewService(cfg config.Config, db *pgxpool.Pool) *Service {
	return &Service{cfg: cfg, db: db}
}

func (s *Service) Login(ctx context.Context, email, password string) (*TokenPair, error) {
	user, passwordHash, failedAttempts, lockedUntil, isActive, err := s.getUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}

	if !isActive {
		return nil, ErrUserBlocked
	}
	if lockedUntil != nil && lockedUntil.After(time.Now()) {
		return nil, ErrUserLocked
	}

	if !ComparePassword(passwordHash, password) {
		_ = s.registerFailedLogin(ctx, user.ID, failedAttempts+1)
		return nil, ErrInvalidCredentials
	}

	if err := s.resetFailedLogins(ctx, user.ID); err != nil {
		return nil, err
	}

	return s.issueTokens(ctx, user)
}

func (s *Service) Refresh(ctx context.Context, refreshToken string) (*TokenPair, error) {
	tokenHash := HashRefreshToken(refreshToken)

	var userID string
	err := s.db.QueryRow(ctx, `
		SELECT user_id
		FROM refresh_tokens
		WHERE token_hash = $1
			AND revoked_at IS NULL
			AND expires_at > NOW()
	`, tokenHash).Scan(&userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrInvalidToken
		}
		return nil, err
	}

	if err := RevokeRefreshToken(ctx, s.db, tokenHash); err != nil {
		return nil, err
	}

	user, err := s.GetUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	return s.issueTokens(ctx, *user)
}

func (s *Service) Logout(ctx context.Context, refreshToken string) error {
	if refreshToken == "" {
		return nil
	}
	return RevokeRefreshToken(ctx, s.db, HashRefreshToken(refreshToken))
}

func (s *Service) GetUserByID(ctx context.Context, userID string) (*User, error) {
	var user User
	var middleName *string

	err := s.db.QueryRow(ctx, `
		SELECT id::text, email, first_name, last_name, middle_name, position
		FROM users
		WHERE id = $1 AND is_active = true
	`, userID).Scan(&user.ID, &user.Email, &user.FirstName, &user.LastName, &middleName, &user.Position)
	if err != nil {
		return nil, err
	}

	user.MiddleName = middleName
	user.Roles, err = s.getRoles(ctx, user.ID)
	if err != nil {
		return nil, err
	}

	return &user, nil
}

func (s *Service) issueTokens(ctx context.Context, user User) (*TokenPair, error) {
	accessToken, accessExpiresAt, err := GenerateAccessToken(s.cfg, user.ID, user.Roles)
	if err != nil {
		return nil, err
	}

	refreshToken, refreshTokenHash, err := GenerateRefreshToken()
	if err != nil {
		return nil, err
	}

	refreshExpiresAt := time.Now().Add(s.cfg.JWTRefreshTTL)
	if err := StoreRefreshToken(ctx, s.db, user.ID, refreshTokenHash, refreshExpiresAt); err != nil {
		return nil, err
	}

	return &TokenPair{
		AccessToken:           accessToken,
		AccessTokenExpiresAt:  accessExpiresAt,
		RefreshToken:          refreshToken,
		RefreshTokenExpiresAt: refreshExpiresAt,
		User:                  user,
	}, nil
}

func (s *Service) getUserByEmail(ctx context.Context, email string) (User, string, int, *time.Time, bool, error) {
	var user User
	var passwordHash string
	var failedAttempts int
	var lockedUntil *time.Time
	var isActive bool
	var middleName *string

	err := s.db.QueryRow(ctx, `
		SELECT id::text, email, first_name, last_name, middle_name, position,
			password_hash, failed_login_attempts, locked_until, is_active
		FROM users
		WHERE lower(email) = lower($1)
	`, strings.TrimSpace(email)).Scan(
		&user.ID, &user.Email, &user.FirstName, &user.LastName, &middleName, &user.Position,
		&passwordHash, &failedAttempts, &lockedUntil, &isActive,
	)
	if err != nil {
		return User{}, "", 0, nil, false, err
	}

	user.MiddleName = middleName
	user.Roles, err = s.getRoles(ctx, user.ID)
	if err != nil {
		return User{}, "", 0, nil, false, err
	}

	return user, passwordHash, failedAttempts, lockedUntil, isActive, nil
}

func (s *Service) getRoles(ctx context.Context, userID string) ([]string, error) {
	rows, err := s.db.Query(ctx, `
		SELECT role
		FROM user_roles
		WHERE user_id = $1
		ORDER BY role
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	roles := make([]string, 0, 2)
	for rows.Next() {
		var role string
		if err := rows.Scan(&role); err != nil {
			return nil, err
		}
		roles = append(roles, role)
	}

	return roles, rows.Err()
}

func (s *Service) registerFailedLogin(ctx context.Context, userID string, attempts int) error {
	var lockedUntil any
	if attempts >= maxFailedLoginAttempts {
		lockedUntil = time.Now().Add(lockoutDuration)
	}

	_, err := s.db.Exec(ctx, `
		UPDATE users
		SET failed_login_attempts = $2,
			locked_until = $3,
			updated_at = NOW()
		WHERE id = $1
	`, userID, attempts, lockedUntil)
	return err
}

func (s *Service) resetFailedLogins(ctx context.Context, userID string) error {
	_, err := s.db.Exec(ctx, `
		UPDATE users
		SET failed_login_attempts = 0,
			locked_until = NULL,
			updated_at = NOW()
		WHERE id = $1
	`, userID)
	return err
}
