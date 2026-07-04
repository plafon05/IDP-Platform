package idp

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"idp-platform/backend/internal/notification"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound          = errors.New("idp not found")
	ErrForbidden         = errors.New("idp access forbidden")
	ErrInvalidInput      = errors.New("invalid idp input")
	ErrInvalidTransition = errors.New("invalid idp status transition")
	ErrIncompleteTasks   = errors.New("idp has incomplete tasks")
	ErrEmployeeNoManager = errors.New("employee has no manager")
	ErrEmployeeInactive  = errors.New("employee is inactive")
)

type Service struct {
	db          *pgxpool.Pool
	publisher   notification.Publisher
	frontendURL string
}

type Access struct {
	UserID  string
	IsHR    bool
	Manager bool
}

type ListParams struct {
	EmployeeID string
	ManagerID  string
	Status     string
	Page       int
	Limit      int
}

type ListResult struct {
	Data []Plan   `json:"data"`
	Meta ListMeta `json:"meta"`
}

type ListMeta struct {
	Total      int `json:"total"`
	Page       int `json:"page"`
	Limit      int `json:"limit"`
	TotalPages int `json:"total_pages"`
}

type CompetencyTarget struct {
	CompetencyID string `json:"competency_id"`
	Name         string `json:"name,omitempty"`
	TargetLevel  int    `json:"target_level"`
	CurrentLevel *int   `json:"current_level,omitempty"`
}

type Plan struct {
	ID             string             `json:"id"`
	EmployeeID     string             `json:"employee_id"`
	EmployeeName   string             `json:"employee_name"`
	ManagerID      string             `json:"manager_id"`
	ManagerName    string             `json:"manager_name"`
	Title          string             `json:"title"`
	Goals          *string            `json:"goals,omitempty"`
	StartDate      string             `json:"start_date"`
	EndDate        string             `json:"end_date"`
	Status         string             `json:"status"`
	CancelReason   *string            `json:"cancel_reason,omitempty"`
	TasksTotal     int                `json:"tasks_total"`
	TasksCompleted int                `json:"tasks_completed"`
	Progress       int                `json:"progress"`
	Competencies   []CompetencyTarget `json:"competencies"`
	CreatedAt      time.Time          `json:"created_at"`
	UpdatedAt      time.Time          `json:"updated_at"`
}

type Input struct {
	EmployeeID   string
	Title        string
	Goals        *string
	StartDate    time.Time
	EndDate      time.Time
	Competencies []CompetencyTarget
}

type StatusInput struct {
	Status  string
	Comment *string
	Reason  *string
}

type AuditEntry struct {
	ID        string          `json:"id"`
	ActorID   *string         `json:"actor_id,omitempty"`
	ActorName string          `json:"actor_name"`
	Action    string          `json:"action"`
	OldValue  json.RawMessage `json:"old_value,omitempty"`
	NewValue  json.RawMessage `json:"new_value,omitempty"`
	CreatedAt time.Time       `json:"created_at"`
}

func NewService(db *pgxpool.Pool, publisher notification.Publisher, frontendURL string) *Service {
	return &Service{db: db, publisher: publisher, frontendURL: strings.TrimRight(frontendURL, "/")}
}

func (s *Service) List(ctx context.Context, access Access, params ListParams) (*ListResult, error) {
	if params.Page < 1 {
		params.Page = 1
	}
	if params.Limit < 1 || params.Limit > 100 {
		params.Limit = 50
	}
	offset := (params.Page - 1) * params.Limit

	var total int
	err := s.db.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM idps i
		WHERE i.archived_at IS NULL
			AND ($1 OR i.employee_id = $2 OR ($3 AND i.manager_id = $2))
			AND (NULLIF($4, '') IS NULL OR i.employee_id = NULLIF($4, '')::uuid)
			AND (NULLIF($5, '') IS NULL OR i.manager_id = NULLIF($5, '')::uuid)
			AND ($6 = '' OR i.status = $6)
	`, access.IsHR, access.UserID, access.Manager, params.EmployeeID, params.ManagerID, params.Status).Scan(&total)
	if err != nil {
		return nil, err
	}

	rows, err := s.db.Query(ctx, `
		SELECT i.id::text,
			i.employee_id::text,
			concat_ws(' ', employee.last_name, employee.first_name, employee.middle_name),
			i.manager_id::text,
			concat_ws(' ', manager.last_name, manager.first_name, manager.middle_name),
			i.title,
			i.goals,
			i.start_date,
			i.end_date,
			i.status,
			i.cancel_reason,
			COUNT(t.id)::int,
			COUNT(t.id) FILTER (WHERE t.status = 'completed')::int,
			COALESCE(ROUND(AVG(t.progress))::int, 0),
			i.created_at,
			i.updated_at
		FROM idps i
		JOIN users employee ON employee.id = i.employee_id
		JOIN users manager ON manager.id = i.manager_id
		LEFT JOIN tasks t ON t.idp_id = i.id AND t.deleted_at IS NULL
		WHERE i.archived_at IS NULL
			AND ($1 OR i.employee_id = $2 OR ($3 AND i.manager_id = $2))
			AND (NULLIF($4, '') IS NULL OR i.employee_id = NULLIF($4, '')::uuid)
			AND (NULLIF($5, '') IS NULL OR i.manager_id = NULLIF($5, '')::uuid)
			AND ($6 = '' OR i.status = $6)
		GROUP BY i.id, employee.id, manager.id
		ORDER BY i.created_at DESC
		LIMIT $7 OFFSET $8
	`, access.IsHR, access.UserID, access.Manager, params.EmployeeID, params.ManagerID, params.Status, params.Limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := &ListResult{
		Data: make([]Plan, 0, params.Limit),
		Meta: ListMeta{
			Total:      total,
			Page:       params.Page,
			Limit:      params.Limit,
			TotalPages: totalPages(total, params.Limit),
		},
	}
	for rows.Next() {
		item, err := scanPlan(rows)
		if err != nil {
			return nil, err
		}
		result.Data = append(result.Data, item)
	}

	return result, rows.Err()
}

func (s *Service) Get(ctx context.Context, access Access, id string) (*Plan, error) {
	item, err := s.get(ctx, id)
	if err != nil {
		return nil, err
	}
	if !canRead(access, item) {
		return nil, ErrForbidden
	}
	return item, nil
}

func (s *Service) Create(ctx context.Context, access Access, input Input) (*Plan, error) {
	if !access.Manager && !access.IsHR {
		return nil, ErrForbidden
	}
	if err := validateInput(input); err != nil {
		return nil, err
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	managerID, employeeActive, err := employeeManager(ctx, tx, input.EmployeeID)
	if err != nil {
		return nil, err
	}
	if !employeeActive {
		return nil, ErrEmployeeInactive
	}
	if managerID == nil {
		return nil, ErrEmployeeNoManager
	}
	if !access.IsHR && *managerID != access.UserID {
		return nil, ErrForbidden
	}

	var id string
	err = tx.QueryRow(ctx, `
		INSERT INTO idps (employee_id, manager_id, title, goals, start_date, end_date)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id::text
	`, input.EmployeeID, *managerID, strings.TrimSpace(input.Title), input.Goals, input.StartDate, input.EndDate).Scan(&id)
	if err != nil {
		return nil, err
	}

	if err := replaceCompetencies(ctx, tx, id, input.Competencies); err != nil {
		return nil, err
	}
	if err := writeAudit(ctx, tx, access.UserID, id, "idp.created", nil, input); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return s.Get(ctx, access, id)
}

func (s *Service) Update(ctx context.Context, access Access, id string, input Input) (*Plan, error) {
	if err := validateInput(input); err != nil {
		return nil, err
	}

	current, err := s.get(ctx, id)
	if err != nil {
		return nil, err
	}
	if !canManage(access, current) {
		return nil, ErrForbidden
	}
	if current.Status != "draft" && current.Status != "active" {
		return nil, ErrInvalidTransition
	}
	if input.EmployeeID != current.EmployeeID {
		return nil, ErrInvalidInput
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `
		UPDATE idps
		SET title = $2,
			goals = $3,
			start_date = $4,
			end_date = $5,
			updated_at = NOW()
		WHERE id = $1 AND archived_at IS NULL
	`, id, strings.TrimSpace(input.Title), input.Goals, input.StartDate, input.EndDate)
	if err != nil {
		return nil, err
	}

	if err := replaceCompetencies(ctx, tx, id, input.Competencies); err != nil {
		return nil, err
	}
	if err := writeAudit(ctx, tx, access.UserID, id, "idp.updated", current, input); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return s.Get(ctx, access, id)
}

func (s *Service) ChangeStatus(ctx context.Context, access Access, id string, input StatusInput) (*Plan, error) {
	current, err := s.get(ctx, id)
	if err != nil {
		return nil, err
	}
	if !canManage(access, current) {
		return nil, ErrForbidden
	}
	if !validTransition(current.Status, input.Status) {
		return nil, ErrInvalidTransition
	}
	if input.Status == "cancelled" && empty(input.Reason) {
		return nil, ErrInvalidInput
	}
	if input.Status == "completed" {
		var incomplete bool
		if err := s.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM tasks WHERE idp_id=$1 AND deleted_at IS NULL AND status IN ('not_started','in_progress'))`, id).Scan(&incomplete); err != nil {
			return nil, err
		}
		if incomplete {
			return nil, ErrIncompleteTasks
		}
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var cancelReason *string
	if input.Status == "cancelled" {
		cancelReason = trimmed(input.Reason)
	}
	_, err = tx.Exec(ctx, `
		UPDATE idps
		SET status = $2,
			cancel_reason = $3,
			updated_at = NOW()
		WHERE id = $1 AND archived_at IS NULL
	`, id, input.Status, cancelReason)
	if err != nil {
		return nil, err
	}

	if input.Status == "completed" && !empty(input.Comment) {
		_, err = tx.Exec(ctx, `
			INSERT INTO comments (author_id, entity_type, entity_id, content)
			VALUES ($1, 'idp', $2, $3)
		`, access.UserID, id, *trimmed(input.Comment))
		if err != nil {
			return nil, err
		}
	}
	if err := writeAudit(ctx, tx, access.UserID, id, "idp.status_changed",
		map[string]string{"status": current.Status},
		map[string]any{"status": input.Status, "cancel_reason": cancelReason},
	); err != nil {
		return nil, err
	}
	if s.publisher != nil {
		var email, firstName string
		if err := tx.QueryRow(ctx, `SELECT email, first_name FROM users WHERE id=$1`, current.EmployeeID).Scan(&email, &firstName); err != nil {
			return nil, err
		}
		data := map[string]string{
			"first_name": firstName,
			"idp_title":  current.Title,
			"status":     input.Status,
			"idp_url":    s.frontendURL + "/plans",
		}
		if cancelReason != nil {
			data["reason"] = *cancelReason
		}
		if err := s.publisher.EnqueueTx(ctx, tx, notification.Job{
			UserID: current.EmployeeID,
			To:     []string{email}, Template: notification.IDPStatusTemplate, Data: data,
		}); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return s.Get(ctx, access, id)
}

func (s *Service) Archive(ctx context.Context, access Access, id string) error {
	if !access.IsHR {
		return ErrForbidden
	}
	current, err := s.get(ctx, id)
	if err != nil {
		return err
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `UPDATE idps SET archived_at = NOW(), updated_at = NOW() WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if err := writeAudit(ctx, tx, access.UserID, id, "idp.archived", current, nil); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Service) Audit(ctx context.Context, access Access, id string) ([]AuditEntry, error) {
	if _, err := s.Get(ctx, access, id); err != nil {
		return nil, err
	}

	rows, err := s.db.Query(ctx, `
		SELECT a.id::text, a.actor_id::text,
			COALESCE(concat_ws(' ', u.last_name, u.first_name, u.middle_name), ''),
			a.action, a.old_value, a.new_value, a.created_at
		FROM audit_logs a
		LEFT JOIN users u ON u.id=a.actor_id
		WHERE a.entity_type = 'idp' AND a.entity_id = $1
		ORDER BY a.created_at, a.id
	`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]AuditEntry, 0)
	for rows.Next() {
		var entry AuditEntry
		if err := rows.Scan(&entry.ID, &entry.ActorID, &entry.ActorName, &entry.Action, &entry.OldValue, &entry.NewValue, &entry.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, entry)
	}
	return result, rows.Err()
}

func (s *Service) get(ctx context.Context, id string) (*Plan, error) {
	row := s.db.QueryRow(ctx, `
		SELECT i.id::text,
			i.employee_id::text,
			concat_ws(' ', employee.last_name, employee.first_name, employee.middle_name),
			i.manager_id::text,
			concat_ws(' ', manager.last_name, manager.first_name, manager.middle_name),
			i.title,
			i.goals,
			i.start_date,
			i.end_date,
			i.status,
			i.cancel_reason,
			COUNT(t.id)::int,
			COUNT(t.id) FILTER (WHERE t.status = 'completed')::int,
			COALESCE(ROUND(AVG(t.progress))::int, 0),
			i.created_at,
			i.updated_at
		FROM idps i
		JOIN users employee ON employee.id = i.employee_id
		JOIN users manager ON manager.id = i.manager_id
		LEFT JOIN tasks t ON t.idp_id = i.id AND t.deleted_at IS NULL
		WHERE i.id = $1 AND i.archived_at IS NULL
		GROUP BY i.id, employee.id, manager.id
	`, id)

	item, err := scanPlan(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	item.Competencies, err = s.competencies(ctx, id)
	if err != nil {
		return nil, err
	}
	return &item, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanPlan(row rowScanner) (Plan, error) {
	var item Plan
	var startDate time.Time
	var endDate time.Time
	err := row.Scan(
		&item.ID,
		&item.EmployeeID,
		&item.EmployeeName,
		&item.ManagerID,
		&item.ManagerName,
		&item.Title,
		&item.Goals,
		&startDate,
		&endDate,
		&item.Status,
		&item.CancelReason,
		&item.TasksTotal,
		&item.TasksCompleted,
		&item.Progress,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	item.StartDate = startDate.Format(time.DateOnly)
	item.EndDate = endDate.Format(time.DateOnly)
	return item, err
}

func (s *Service) competencies(ctx context.Context, id string) ([]CompetencyTarget, error) {
	rows, err := s.db.Query(ctx, `
		SELECT ic.competency_id::text, c.name, ic.target_level, ic.current_level
		FROM idp_competencies ic
		JOIN competencies c ON c.id = ic.competency_id
		WHERE ic.idp_id = $1
		ORDER BY c.name
	`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]CompetencyTarget, 0)
	for rows.Next() {
		var item CompetencyTarget
		if err := rows.Scan(&item.CompetencyID, &item.Name, &item.TargetLevel, &item.CurrentLevel); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func employeeManager(ctx context.Context, tx pgx.Tx, employeeID string) (*string, bool, error) {
	var managerID *string
	var active bool
	err := tx.QueryRow(ctx, `
		SELECT manager_id::text, is_active
		FROM users
		WHERE id = $1
	`, employeeID).Scan(&managerID, &active)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false, ErrNotFound
	}
	return managerID, active, err
}

func replaceCompetencies(ctx context.Context, tx pgx.Tx, id string, items []CompetencyTarget) error {
	if _, err := tx.Exec(ctx, `DELETE FROM idp_competencies WHERE idp_id = $1`, id); err != nil {
		return err
	}

	seen := make(map[string]bool, len(items))
	for _, item := range items {
		if strings.TrimSpace(item.CompetencyID) == "" || item.TargetLevel < 1 || item.TargetLevel > 4 || seen[item.CompetencyID] {
			return ErrInvalidInput
		}
		if item.CurrentLevel != nil && (*item.CurrentLevel < 1 || *item.CurrentLevel > 4) {
			return ErrInvalidInput
		}
		seen[item.CompetencyID] = true

		tag, err := tx.Exec(ctx, `
			INSERT INTO idp_competencies (idp_id, competency_id, target_level, current_level)
			SELECT $1, id, $3, $4
			FROM competencies
			WHERE id = $2 AND is_active = true
		`, id, item.CompetencyID, item.TargetLevel, item.CurrentLevel)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return ErrInvalidInput
		}
	}
	return nil
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

	_, err = tx.Exec(ctx, `
		INSERT INTO audit_logs (actor_id, entity_type, entity_id, action, old_value, new_value)
		VALUES ($1, 'idp', $2, $3, $4, $5)
	`, actorID, entityID, action, oldJSON, newJSON)
	return err
}

func marshalNullable(value any) ([]byte, error) {
	if value == nil {
		return nil, nil
	}
	return json.Marshal(value)
}

func validateInput(input Input) error {
	if strings.TrimSpace(input.EmployeeID) == "" ||
		strings.TrimSpace(input.Title) == "" ||
		len([]rune(strings.TrimSpace(input.Title))) > 300 ||
		(input.Goals != nil && len([]rune(*input.Goals)) > 10000) ||
		input.StartDate.IsZero() ||
		input.EndDate.IsZero() ||
		input.EndDate.Before(input.StartDate) {
		return ErrInvalidInput
	}
	return nil
}

func canRead(access Access, item *Plan) bool {
	return access.IsHR || item.EmployeeID == access.UserID || (access.Manager && item.ManagerID == access.UserID)
}

func canManage(access Access, item *Plan) bool {
	return access.IsHR || (access.Manager && item.ManagerID == access.UserID)
}

func validTransition(current, next string) bool {
	switch current {
	case "draft":
		return next == "active" || next == "cancelled"
	case "active":
		return next == "completed" || next == "cancelled"
	default:
		return false
	}
}

func empty(value *string) bool {
	return value == nil || strings.TrimSpace(*value) == ""
}

func trimmed(value *string) *string {
	if value == nil {
		return nil
	}
	result := strings.TrimSpace(*value)
	return &result
}

func totalPages(total, limit int) int {
	if total == 0 {
		return 0
	}
	return (total + limit - 1) / limit
}
