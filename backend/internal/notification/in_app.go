package notification

import (
	"context"
	"errors"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotificationNotFound = errors.New("notification not found")

type InAppNotification struct {
	ID        string     `json:"id"`
	Kind      string     `json:"kind"`
	Title     string     `json:"title"`
	Message   string     `json:"message"`
	ActionURL *string    `json:"action_url"`
	ReadAt    *time.Time `json:"read_at"`
	CreatedAt time.Time  `json:"created_at"`
}

type InAppList struct {
	Data        []InAppNotification `json:"data"`
	UnreadCount int                 `json:"unread_count"`
}

type InAppService struct{ db *pgxpool.Pool }

func NewInAppService(db *pgxpool.Pool) *InAppService { return &InAppService{db: db} }

func (s *InAppService) List(ctx context.Context, userID, kind string, unreadOnly, oldestFirst bool, dateFrom, dateTo *time.Time) (InAppList, error) {
	query := `SELECT id::text, kind, title, message, action_url, read_at, created_at
		FROM in_app_notifications WHERE user_id=$1`
	args := []any{userID}
	if kind != "" {
		args = append(args, kind)
		query += ` AND kind=$` + strconv.Itoa(len(args))
	}
	if unreadOnly {
		query += ` AND read_at IS NULL`
	}
	if dateFrom != nil {
		args = append(args, *dateFrom)
		query += ` AND created_at >= $` + strconv.Itoa(len(args))
	}
	if dateTo != nil {
		args = append(args, dateTo.AddDate(0, 0, 1))
		query += ` AND created_at < $` + strconv.Itoa(len(args))
	}
	if oldestFirst {
		query += ` ORDER BY created_at ASC`
	} else {
		query += ` ORDER BY created_at DESC`
	}
	query += ` LIMIT 100`

	rows, err := s.db.Query(ctx, query, args...)
	if err != nil {
		return InAppList{}, err
	}
	defer rows.Close()
	result := InAppList{Data: make([]InAppNotification, 0)}
	for rows.Next() {
		var item InAppNotification
		if err := rows.Scan(&item.ID, &item.Kind, &item.Title, &item.Message, &item.ActionURL, &item.ReadAt, &item.CreatedAt); err != nil {
			return InAppList{}, err
		}
		result.Data = append(result.Data, item)
	}
	if err := rows.Err(); err != nil {
		return InAppList{}, err
	}
	if err := s.db.QueryRow(ctx, `SELECT COUNT(*) FROM in_app_notifications WHERE user_id=$1 AND read_at IS NULL`, userID).Scan(&result.UnreadCount); err != nil {
		return InAppList{}, err
	}
	return result, nil
}

func (s *InAppService) MarkRead(ctx context.Context, userID, id string) error {
	tag, err := s.db.Exec(ctx, `UPDATE in_app_notifications SET read_at=COALESCE(read_at,NOW()) WHERE id=$1 AND user_id=$2`, id, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotificationNotFound
	}
	return nil
}

func (s *InAppService) MarkAllRead(ctx context.Context, userID string) error {
	_, err := s.db.Exec(ctx, `UPDATE in_app_notifications SET read_at=NOW() WHERE user_id=$1 AND read_at IS NULL`, userID)
	return err
}

type inAppContent struct{ kind, title, message, actionURL string }

func contentForJob(job Job) (inAppContent, bool) {
	data := job.Data
	content := inAppContent{kind: job.Template, actionURL: appPath(data["plans_url"])}
	switch job.Template {
	case IDPStatusTemplate:
		content.title = "Статус ИПР изменён"
		content.message = "План «" + data["idp_title"] + "» перешёл в статус «" + statusLabel(data["status"]) + "»."
		content.actionURL = appPath(data["idp_url"])
	case TaskChangedTemplate:
		if data["event"] == "created" {
			content.title = "Назначена новая задача"
			content.message = "Вам назначена задача «" + data["task_title"] + "»."
		} else {
			content.title = "Задача изменена"
			content.message = "Руководитель изменил задачу «" + data["task_title"] + "»."
		}
	case TaskReviewTemplate:
		content.title = "Задача оценена"
		content.message = "Руководитель добавил оценку к задаче «" + data["task_title"] + "»."
	case CommentCreatedTemplate:
		content.title = "Новый комментарий"
		content.message = data["author_name"] + " оставил комментарий к «" + data["entity_title"] + "»."
	case TaskDeadlineTemplate:
		if data["kind"] == "overdue" {
			content.title = "Задача просрочена"
		} else {
			content.title = "Приближается срок задачи"
		}
		content.message = "Задача «" + data["task_title"] + "», срок: " + data["due_date"] + "."
	default:
		return inAppContent{}, false
	}
	return content, job.UserID != "" && content.title != ""
}

func insertInApp(ctx context.Context, db queryExecer, job Job) error {
	content, ok := contentForJob(job)
	if !ok {
		return nil
	}
	var actionURL any
	if content.actionURL != "" {
		actionURL = content.actionURL
	}
	_, err := db.Exec(ctx, `INSERT INTO in_app_notifications (user_id,kind,title,message,action_url) VALUES ($1,$2,$3,$4,$5)`,
		job.UserID, content.kind, content.title, content.message, actionURL)
	return err
}

func appPath(raw string) string {
	if raw == "" {
		return ""
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Path == "" {
		return ""
	}
	if parsed.RawQuery != "" {
		return parsed.Path + "?" + parsed.RawQuery
	}
	return parsed.Path
}

func statusLabel(value string) string {
	labels := map[string]string{"draft": "Черновик", "active": "Активен", "completed": "Завершён", "cancelled": "Отменён"}
	if label := labels[strings.ToLower(value)]; label != "" {
		return label
	}
	return value
}
