package comments

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"idp-platform/backend/internal/idp"
	"idp-platform/backend/internal/notification"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const editWindow = 10 * time.Minute

var (
	ErrNotFound     = errors.New("comment or entity not found")
	ErrForbidden    = errors.New("comment access forbidden")
	ErrInvalidInput = errors.New("invalid comment input")
	ErrEditExpired  = errors.New("comment edit window expired")
)

type Service struct {
	db          *pgxpool.Pool
	publisher   notification.Publisher
	frontendURL string
}

type Comment struct {
	ID           string    `json:"id"`
	EntityType   string    `json:"entity_type"`
	EntityID     string    `json:"entity_id"`
	AuthorID     string    `json:"author_id"`
	AuthorName   string    `json:"author_name"`
	AuthorAvatar *string   `json:"author_avatar,omitempty"`
	Content      string    `json:"content"`
	IsDeleted    bool      `json:"is_deleted"`
	CanEdit      bool      `json:"can_edit"`
	CanDelete    bool      `json:"can_delete"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type entityAccess struct {
	EmployeeID string
	ManagerID  string
	IDPID      string
}

func NewService(db *pgxpool.Pool, publisher notification.Publisher, frontendURL string) *Service {
	return &Service{db: db, publisher: publisher, frontendURL: strings.TrimRight(frontendURL, "/")}
}

func (s *Service) List(ctx context.Context, access idp.Access, entityType, entityID string) ([]Comment, error) {
	entity, err := s.entity(ctx, entityType, entityID)
	if err != nil {
		return nil, err
	}
	if !canRead(access, entity) {
		return nil, ErrForbidden
	}
	rows, err := s.db.Query(ctx, `
		SELECT c.id::text, c.entity_type, c.entity_id::text, c.author_id::text,
			concat_ws(' ', u.last_name, u.first_name, u.middle_name), u.avatar_url,
			c.content, c.is_deleted, c.created_at, c.updated_at
		FROM comments c JOIN users u ON u.id=c.author_id
		WHERE c.entity_type=$1 AND c.entity_id=$2
		ORDER BY c.created_at, c.id
	`, entityType, entityID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]Comment, 0)
	for rows.Next() {
		item, err := scanComment(rows, access.UserID)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *Service) Create(ctx context.Context, access idp.Access, entityType, entityID, content string) (*Comment, error) {
	content = strings.TrimSpace(content)
	if !validContent(content) {
		return nil, ErrInvalidInput
	}
	entity, err := s.entity(ctx, entityType, entityID)
	if err != nil {
		return nil, err
	}
	if !canRead(access, entity) {
		return nil, ErrForbidden
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	var id string
	if err := tx.QueryRow(ctx, `INSERT INTO comments (author_id, entity_type, entity_id, content)
		VALUES ($1,$2,$3,$4) RETURNING id::text`, access.UserID, entityType, entityID, content).Scan(&id); err != nil {
		return nil, err
	}
	if err := writeAudit(ctx, tx, access.UserID, id, "comment.created", nil, map[string]any{"entity_type": entityType, "entity_id": entityID, "content": content}); err != nil {
		return nil, err
	}
	if err := s.enqueueCommentNotifications(ctx, tx, access.UserID, entity, entityType, entityID, content); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.get(ctx, access, id)
}

func (s *Service) enqueueCommentNotifications(ctx context.Context, tx pgx.Tx, authorID string, entity *entityAccess, entityType, entityID, content string) error {
	if s.publisher == nil {
		return nil
	}
	var authorName, entityTitle string
	if err := tx.QueryRow(ctx, `SELECT concat_ws(' ', last_name, first_name) FROM users WHERE id=$1`, authorID).Scan(&authorName); err != nil {
		return err
	}
	query := `SELECT title FROM idps WHERE id=$1`
	if entityType == "task" {
		query = `SELECT title FROM tasks WHERE id=$1`
	}
	if err := tx.QueryRow(ctx, query, entityID).Scan(&entityTitle); err != nil {
		return err
	}
	rows, err := tx.Query(ctx, `
		SELECT id::text, email FROM users
		WHERE id IN ($1,$2) AND id<>$3 AND is_active=true
	`, entity.EmployeeID, entity.ManagerID, authorID)
	if err != nil {
		return err
	}
	type recipient struct{ id, email string }
	var recipients []recipient
	for rows.Next() {
		var item recipient
		if err := rows.Scan(&item.id, &item.email); err != nil {
			rows.Close()
			return err
		}
		recipients = append(recipients, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()

	excerpt := []rune(content)
	if len(excerpt) > 240 {
		excerpt = append(excerpt[:237], '.', '.', '.')
	}
	for _, recipient := range recipients {
		plansURL := s.frontendURL + "/plans?id=" + entity.IDPID + "&section=comments"
		if entityType == "task" {
			plansURL = s.frontendURL + "/plans?id=" + entity.IDPID + "&section=tasks&task=" + entityID + "&task_section=comments"
		}
		if err := s.publisher.EnqueueTx(ctx, tx, notification.Job{
			UserID: recipient.id, To: []string{recipient.email}, Template: notification.CommentCreatedTemplate,
			Data: map[string]string{
				"author_name": authorName, "entity_type": entityType, "entity_title": entityTitle,
				"excerpt": string(excerpt), "plans_url": plansURL,
			},
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) Update(ctx context.Context, access idp.Access, id, content string) (*Comment, error) {
	content = strings.TrimSpace(content)
	if !validContent(content) {
		return nil, ErrInvalidInput
	}
	current, entity, err := s.getWithEntity(ctx, id)
	if err != nil {
		return nil, err
	}
	if !canRead(access, entity) || current.AuthorID != access.UserID || current.IsDeleted {
		return nil, ErrForbidden
	}
	if time.Since(current.CreatedAt) > editWindow {
		return nil, ErrEditExpired
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `UPDATE comments SET content=$2, updated_at=NOW() WHERE id=$1 AND is_deleted=false`, id, content); err != nil {
		return nil, err
	}
	if err := writeAudit(ctx, tx, access.UserID, id, "comment.updated", map[string]string{"content": current.Content}, map[string]string{"content": content}); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.get(ctx, access, id)
}

func (s *Service) Delete(ctx context.Context, access idp.Access, id string) error {
	current, entity, err := s.getWithEntity(ctx, id)
	if err != nil {
		return err
	}
	if !canRead(access, entity) || current.AuthorID != access.UserID || current.IsDeleted {
		return ErrForbidden
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `UPDATE comments SET content='', is_deleted=true, updated_at=NOW() WHERE id=$1`, id); err != nil {
		return err
	}
	if err := writeAudit(ctx, tx, access.UserID, id, "comment.deleted", map[string]string{"content": current.Content}, nil); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Service) get(ctx context.Context, access idp.Access, id string) (*Comment, error) {
	item, entity, err := s.getWithEntity(ctx, id)
	if err != nil {
		return nil, err
	}
	if !canRead(access, entity) {
		return nil, ErrForbidden
	}
	item.CanEdit = item.AuthorID == access.UserID && !item.IsDeleted && time.Since(item.CreatedAt) <= editWindow
	item.CanDelete = item.AuthorID == access.UserID && !item.IsDeleted
	if item.IsDeleted {
		item.Content = "Комментарий удалён"
	}
	return item, nil
}

func (s *Service) getWithEntity(ctx context.Context, id string) (*Comment, *entityAccess, error) {
	row := s.db.QueryRow(ctx, `
		SELECT c.id::text, c.entity_type, c.entity_id::text, c.author_id::text,
			concat_ws(' ', u.last_name, u.first_name, u.middle_name), u.avatar_url,
			c.content, c.is_deleted, c.created_at, c.updated_at
		FROM comments c JOIN users u ON u.id=c.author_id WHERE c.id=$1
	`, id)
	item, err := scanComment(row, "")
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, ErrNotFound
	}
	if err != nil {
		return nil, nil, err
	}
	entity, err := s.entity(ctx, item.EntityType, item.EntityID)
	return &item, entity, err
}

func (s *Service) entity(ctx context.Context, entityType, entityID string) (*entityAccess, error) {
	var item entityAccess
	var err error
	switch entityType {
	case "idp":
		err = s.db.QueryRow(ctx, `SELECT employee_id::text, manager_id::text, id::text FROM idps WHERE id=$1 AND archived_at IS NULL`, entityID).Scan(&item.EmployeeID, &item.ManagerID, &item.IDPID)
	case "task":
		err = s.db.QueryRow(ctx, `SELECT i.employee_id::text, i.manager_id::text, i.id::text FROM tasks t JOIN idps i ON i.id=t.idp_id WHERE t.id=$1 AND t.deleted_at IS NULL AND i.archived_at IS NULL`, entityID).Scan(&item.EmployeeID, &item.ManagerID, &item.IDPID)
	default:
		return nil, ErrInvalidInput
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &item, err
}

type rowScanner interface{ Scan(...any) error }

func scanComment(row rowScanner, currentUserID string) (Comment, error) {
	var item Comment
	err := row.Scan(&item.ID, &item.EntityType, &item.EntityID, &item.AuthorID, &item.AuthorName,
		&item.AuthorAvatar, &item.Content, &item.IsDeleted, &item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		return item, err
	}
	item.CanEdit = item.AuthorID == currentUserID && !item.IsDeleted && time.Since(item.CreatedAt) <= editWindow
	item.CanDelete = item.AuthorID == currentUserID && !item.IsDeleted
	if item.IsDeleted {
		item.Content = "Комментарий удалён"
	}
	return item, nil
}

func validContent(content string) bool {
	length := len([]rune(content))
	return length > 0 && length <= 5000
}
func canRead(access idp.Access, entity *entityAccess) bool {
	return access.IsHR || access.UserID == entity.EmployeeID || (access.Manager && access.UserID == entity.ManagerID)
}

func writeAudit(ctx context.Context, tx pgx.Tx, actorID, entityID, action string, oldValue, newValue any) error {
	oldJSON, err := marshalNullable(oldValue)
	if err != nil {
		return err
	}
	newJSON, err := marshalNullable(newValue)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO audit_logs (actor_id, entity_type, entity_id, action, old_value, new_value) VALUES ($1,'comment',$2,$3,$4,$5)`, actorID, entityID, action, oldJSON, newJSON)
	return err
}

func marshalNullable(value any) ([]byte, error) {
	if value == nil {
		return nil, nil
	}
	return json.Marshal(value)
}
