package notification

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrInvalidUnsubscribeToken = errors.New("invalid unsubscribe token")

type Preferences struct {
	EmailEnabled bool `json:"email_enabled"`
	IDPUpdates   bool `json:"idp_updates"`
	TaskUpdates  bool `json:"task_updates"`
	Comments     bool `json:"comments"`
	Reminders    bool `json:"reminders"`
}

type PreferencesService struct {
	db     *pgxpool.Pool
	secret []byte
}

func NewPreferencesService(db *pgxpool.Pool, secret string) *PreferencesService {
	return &PreferencesService{db: db, secret: []byte(secret)}
}

func (s *PreferencesService) Get(ctx context.Context, userID string) (Preferences, error) {
	result := Preferences{EmailEnabled: true, IDPUpdates: true, TaskUpdates: true, Comments: true, Reminders: true}
	err := s.db.QueryRow(ctx, `
		SELECT COALESCE(email_enabled,true),COALESCE(idp_updates,true),COALESCE(task_updates,true),
			COALESCE(comments,true),COALESCE(reminders,true)
		FROM users u LEFT JOIN notification_preferences p ON p.user_id=u.id WHERE u.id=$1
	`, userID).Scan(&result.EmailEnabled, &result.IDPUpdates, &result.TaskUpdates, &result.Comments, &result.Reminders)
	return result, err
}

func (s *PreferencesService) Update(ctx context.Context, userID string, value Preferences) (Preferences, error) {
	_, err := s.db.Exec(ctx, `
		INSERT INTO notification_preferences (user_id,email_enabled,idp_updates,task_updates,comments,reminders)
		VALUES ($1,$2,$3,$4,$5,$6)
		ON CONFLICT (user_id) DO UPDATE SET email_enabled=$2,idp_updates=$3,task_updates=$4,
			comments=$5,reminders=$6,updated_at=NOW()
	`, userID, value.EmailEnabled, value.IDPUpdates, value.TaskUpdates, value.Comments, value.Reminders)
	return value, err
}

func (s *PreferencesService) Unsubscribe(ctx context.Context, token string) error {
	userID, err := verifyUnsubscribeToken(token, s.secret)
	if err != nil {
		return err
	}
	tag, err := s.db.Exec(ctx, `
		INSERT INTO notification_preferences (user_id,email_enabled)
		SELECT id,false FROM users WHERE id::text=$1
		ON CONFLICT (user_id) DO UPDATE SET email_enabled=false,updated_at=NOW()
	`, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrInvalidUnsubscribeToken
	}
	return nil
}

func signUnsubscribeToken(userID string, secret []byte) string {
	payload := base64.RawURLEncoding.EncodeToString([]byte(userID))
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(payload))
	return payload + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func verifyUnsubscribeToken(token string, secret []byte) (string, error) {
	payload, signature, ok := strings.Cut(token, ".")
	if !ok {
		return "", ErrInvalidUnsubscribeToken
	}
	want, err := base64.RawURLEncoding.DecodeString(signature)
	if err != nil {
		return "", ErrInvalidUnsubscribeToken
	}
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(payload))
	if !hmac.Equal(want, mac.Sum(nil)) {
		return "", ErrInvalidUnsubscribeToken
	}
	decoded, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil || len(decoded) == 0 {
		return "", ErrInvalidUnsubscribeToken
	}
	return string(decoded), nil
}

func preferenceColumn(template string) string {
	switch template {
	case IDPStatusTemplate:
		return "idp_updates"
	case TaskChangedTemplate, TaskReviewTemplate:
		return "task_updates"
	case CommentCreatedTemplate:
		return "comments"
	case TaskDeadlineTemplate:
		return "reminders"
	default:
		return ""
	}
}
