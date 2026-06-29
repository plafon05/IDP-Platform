package tasks

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"slices"
	"strings"
	"time"

	"idp-platform/backend/internal/idp"
	"idp-platform/backend/internal/notification"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound     = errors.New("task not found")
	ErrForbidden    = errors.New("task access forbidden")
	ErrInvalidInput = errors.New("invalid task input")
	ErrIDPState     = errors.New("idp state does not allow task changes")
)

type Service struct {
	db          *pgxpool.Pool
	publisher   notification.Publisher
	frontendURL string
}

type Reference struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Resource struct {
	ID    string `json:"id,omitempty"`
	Title string `json:"title"`
	URL   string `json:"url"`
}

type Task struct {
	ID             string      `json:"id"`
	IDPID          string      `json:"idp_id"`
	Title          string      `json:"title"`
	Description    *string     `json:"description,omitempty"`
	Category       *Reference  `json:"category,omitempty"`
	Priority       string      `json:"priority"`
	DueDate        *string     `json:"due_date,omitempty"`
	Status         string      `json:"status"`
	Progress       int         `json:"progress"`
	ManagerRating  *string     `json:"manager_rating,omitempty"`
	ManagerComment *string     `json:"manager_comment,omitempty"`
	Competencies   []Reference `json:"competencies"`
	Tags           []Reference `json:"tags"`
	Resources      []Resource  `json:"resources"`
	CreatedAt      time.Time   `json:"created_at"`
	UpdatedAt      time.Time   `json:"updated_at"`
}

type Input struct {
	Title          string
	Description    *string
	CategoryID     *string
	Priority       string
	DueDate        *time.Time
	Status         string
	Progress       int
	ManagerRating  *string
	ManagerComment *string
	CompetencyIDs  []string
	TagIDs         []string
	Resources      []Resource
}

type ProgressInput struct {
	Status   string
	Progress int
}

type ListParams struct {
	Status       string
	Priority     string
	CompetencyID string
	Sort         string
	Order        string
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

type planAccess struct {
	EmployeeID string
	ManagerID  string
	Status     string
	StartDate  time.Time
	EndDate    time.Time
}

func NewService(db *pgxpool.Pool, publisher notification.Publisher, frontendURL string) *Service {
	return &Service{db: db, publisher: publisher, frontendURL: strings.TrimRight(frontendURL, "/")}
}

func (s *Service) List(ctx context.Context, access idp.Access, idpID string, params ListParams) ([]Task, error) {
	plan, err := s.plan(ctx, idpID)
	if err != nil {
		return nil, err
	}
	if !canRead(access, plan) {
		return nil, ErrForbidden
	}

	orderBy, err := taskOrderBy(params.Sort, params.Order)
	if err != nil {
		return nil, err
	}
	if (params.Status != "" && !validStatus(params.Status)) ||
		(params.Priority != "" && !oneOf(params.Priority, "low", "medium", "high")) {
		return nil, ErrInvalidInput
	}

	rows, err := s.db.Query(ctx, taskSelect+`
		WHERE t.idp_id = $1 AND t.deleted_at IS NULL
			AND ($2='' OR t.status=$2)
			AND ($3='' OR t.priority=$3)
			AND ($4='' OR EXISTS(SELECT 1 FROM task_competencies tc WHERE tc.task_id=t.id AND tc.competency_id=NULLIF($4,'')::uuid))
		ORDER BY `+orderBy+`, t.created_at, t.id
	`, idpID, params.Status, params.Priority, params.CompetencyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]Task, 0)
	for rows.Next() {
		item, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		if err := s.loadRelations(ctx, &item); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *Service) Get(ctx context.Context, access idp.Access, taskID string) (*Task, error) {
	item, plan, err := s.get(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if !canRead(access, plan) {
		return nil, ErrForbidden
	}
	return item, nil
}

func (s *Service) Create(ctx context.Context, access idp.Access, idpID string, input Input) (*Task, error) {
	plan, err := s.plan(ctx, idpID)
	if err != nil {
		return nil, err
	}
	if !canManage(access, plan) {
		return nil, ErrForbidden
	}
	if !editablePlan(plan.Status) {
		return nil, ErrIDPState
	}
	input = normalizeNewTask(input)
	if input.Priority == "" {
		input.Priority = "medium"
	}
	if err := validateInput(input, plan); err != nil {
		return nil, err
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if err := validateCategory(ctx, tx, input.CategoryID); err != nil {
		return nil, err
	}

	var id string
	err = tx.QueryRow(ctx, `
		INSERT INTO tasks (idp_id, title, description, category_id, priority, due_date, status, progress, manager_rating, manager_comment)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id::text
	`, idpID, strings.TrimSpace(input.Title), trimmed(input.Description), trimmed(input.CategoryID), input.Priority,
		input.DueDate, input.Status, input.Progress, trimmed(input.ManagerRating), trimmed(input.ManagerComment)).Scan(&id)
	if err != nil {
		return nil, err
	}
	if err := replaceRelations(ctx, tx, idpID, id, input); err != nil {
		return nil, err
	}
	if err := writeAudit(ctx, tx, access.UserID, id, "task.created", nil, input); err != nil {
		return nil, err
	}
	if plan.Status == "active" {
		if err := s.enqueueTaskChange(ctx, tx, plan.EmployeeID, "created", strings.TrimSpace(input.Title), input.DueDate); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.Get(ctx, access, id)
}

func (s *Service) Update(ctx context.Context, access idp.Access, taskID string, input Input) (*Task, error) {
	current, plan, err := s.get(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if !canManage(access, plan) {
		return nil, ErrForbidden
	}
	if !editablePlan(plan.Status) {
		return nil, ErrIDPState
	}
	input, err = normalizeManagerUpdate(current, input)
	if err != nil {
		return nil, err
	}
	if err := validateInput(input, plan); err != nil {
		return nil, err
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if err := validateCategory(ctx, tx, input.CategoryID); err != nil {
		return nil, err
	}
	_, err = tx.Exec(ctx, `
		UPDATE tasks SET title=$2, description=$3, category_id=$4, priority=$5, due_date=$6,
			manager_rating=$7, manager_comment=$8, updated_at=NOW()
		WHERE id=$1 AND deleted_at IS NULL
	`, taskID, strings.TrimSpace(input.Title), trimmed(input.Description), trimmed(input.CategoryID), input.Priority,
		input.DueDate, trimmed(input.ManagerRating), trimmed(input.ManagerComment))
	if err != nil {
		return nil, err
	}
	if err := replaceRelations(ctx, tx, current.IDPID, taskID, input); err != nil {
		return nil, err
	}
	if err := writeAudit(ctx, tx, access.UserID, taskID, "task.updated", current, input); err != nil {
		return nil, err
	}
	if plan.Status == "active" && taskDefinitionChanged(current, input) {
		if err := s.enqueueTaskChange(ctx, tx, plan.EmployeeID, "updated", strings.TrimSpace(input.Title), input.DueDate); err != nil {
			return nil, err
		}
	}
	if s.publisher != nil && reviewChanged(current, input) {
		var email, firstName string
		if err := tx.QueryRow(ctx, `SELECT email, first_name FROM users WHERE id=$1`, plan.EmployeeID).Scan(&email, &firstName); err != nil {
			return nil, err
		}
		data := map[string]string{
			"first_name": firstName,
			"task_title": strings.TrimSpace(input.Title),
			"plans_url":  s.frontendURL + "/plans",
		}
		if rating := trimmed(input.ManagerRating); rating != nil {
			data["rating"] = *rating
		}
		if comment := trimmed(input.ManagerComment); comment != nil {
			data["comment"] = *comment
		}
		if err := s.publisher.EnqueueTx(ctx, tx, notification.Job{
			UserID: plan.EmployeeID,
			To:     []string{email}, Template: notification.TaskReviewTemplate, Data: data,
		}); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.Get(ctx, access, taskID)
}

func (s *Service) enqueueTaskChange(ctx context.Context, tx pgx.Tx, employeeID, event, title string, dueDate *time.Time) error {
	if s.publisher == nil {
		return nil
	}
	var email string
	if err := tx.QueryRow(ctx, `SELECT email FROM users WHERE id=$1`, employeeID).Scan(&email); err != nil {
		return err
	}
	data := map[string]string{
		"event": event, "task_title": title, "plans_url": s.frontendURL + "/plans",
	}
	if dueDate != nil {
		data["due_date"] = dueDate.Format(time.DateOnly)
	}
	return s.publisher.EnqueueTx(ctx, tx, notification.Job{
		UserID: employeeID,
		To:     []string{email}, Template: notification.TaskChangedTemplate, Data: data,
	})
}

func (s *Service) UpdateProgress(ctx context.Context, access idp.Access, taskID string, input ProgressInput) (*Task, error) {
	current, plan, err := s.get(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if access.UserID != plan.EmployeeID {
		return nil, ErrForbidden
	}
	if plan.Status != "active" {
		return nil, ErrIDPState
	}
	if !validStatus(input.Status) || input.Progress < 0 || input.Progress > 100 ||
		(input.Status == "completed" && input.Progress != 100) {
		return nil, ErrInvalidInput
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	_, err = tx.Exec(ctx, `
		UPDATE tasks SET status=$2, progress=$3, updated_at=NOW()
		WHERE id=$1 AND deleted_at IS NULL
	`, taskID, input.Status, input.Progress)
	if err != nil {
		return nil, err
	}
	if err := writeAudit(ctx, tx, access.UserID, taskID, "task.progress_changed", current, input); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.Get(ctx, access, taskID)
}

func (s *Service) Delete(ctx context.Context, access idp.Access, taskID string) error {
	current, plan, err := s.get(ctx, taskID)
	if err != nil {
		return err
	}
	if !canManage(access, plan) {
		return ErrForbidden
	}
	if !editablePlan(plan.Status) {
		return ErrIDPState
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `UPDATE tasks SET deleted_at=NOW(), updated_at=NOW() WHERE id=$1`, taskID); err != nil {
		return err
	}
	if err := writeAudit(ctx, tx, access.UserID, taskID, "task.deleted", current, nil); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Service) Audit(ctx context.Context, access idp.Access, taskID string) ([]AuditEntry, error) {
	_, plan, err := s.get(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if !canRead(access, plan) {
		return nil, ErrForbidden
	}
	rows, err := s.db.Query(ctx, `
		SELECT a.id::text, a.actor_id::text,
			COALESCE(concat_ws(' ', u.last_name, u.first_name, u.middle_name), ''),
			a.action, a.old_value, a.new_value, a.created_at
		FROM audit_logs a LEFT JOIN users u ON u.id=a.actor_id
		WHERE a.entity_type='task' AND a.entity_id=$1
		ORDER BY a.created_at, a.id
	`, taskID)
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

const taskSelect = `
	SELECT t.id::text, t.idp_id::text, t.title, t.description,
		t.category_id::text, c.name, t.priority, t.due_date, t.status, t.progress,
		t.manager_rating, t.manager_comment, t.created_at, t.updated_at
	FROM tasks t
	LEFT JOIN task_categories c ON c.id = t.category_id
`

type rowScanner interface{ Scan(...any) error }

func scanTask(row rowScanner) (Task, error) {
	var item Task
	var categoryID, categoryName *string
	var dueDate *time.Time
	err := row.Scan(&item.ID, &item.IDPID, &item.Title, &item.Description, &categoryID, &categoryName,
		&item.Priority, &dueDate, &item.Status, &item.Progress, &item.ManagerRating, &item.ManagerComment,
		&item.CreatedAt, &item.UpdatedAt)
	if categoryID != nil {
		item.Category = &Reference{ID: *categoryID, Name: *categoryName}
	}
	if dueDate != nil {
		formatted := dueDate.Format(time.DateOnly)
		item.DueDate = &formatted
	}
	return item, err
}

func (s *Service) get(ctx context.Context, taskID string) (*Task, *planAccess, error) {
	row := s.db.QueryRow(ctx, taskSelect+` WHERE t.id=$1 AND t.deleted_at IS NULL`, taskID)
	item, err := scanTask(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, ErrNotFound
	}
	if err != nil {
		return nil, nil, err
	}
	plan, err := s.plan(ctx, item.IDPID)
	if err != nil {
		return nil, nil, err
	}
	if err := s.loadRelations(ctx, &item); err != nil {
		return nil, nil, err
	}
	return &item, plan, nil
}

func (s *Service) plan(ctx context.Context, id string) (*planAccess, error) {
	var plan planAccess
	err := s.db.QueryRow(ctx, `
		SELECT employee_id::text, manager_id::text, status, start_date, end_date
		FROM idps WHERE id=$1 AND archived_at IS NULL
	`, id).Scan(&plan.EmployeeID, &plan.ManagerID, &plan.Status, &plan.StartDate, &plan.EndDate)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &plan, err
}

func (s *Service) loadRelations(ctx context.Context, item *Task) error {
	var err error
	item.Competencies, err = queryReferences(ctx, s.db, `
		SELECT c.id::text, c.name FROM task_competencies tc JOIN competencies c ON c.id=tc.competency_id
		WHERE tc.task_id=$1 ORDER BY c.name`, item.ID)
	if err != nil {
		return err
	}
	item.Tags, err = queryReferences(ctx, s.db, `
		SELECT t.id::text, t.name FROM task_tags tt JOIN tags t ON t.id=tt.tag_id
		WHERE tt.task_id=$1 ORDER BY t.name`, item.ID)
	if err != nil {
		return err
	}
	rows, err := s.db.Query(ctx, `SELECT id::text, title, url FROM task_resources WHERE task_id=$1 ORDER BY id`, item.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	item.Resources = make([]Resource, 0)
	for rows.Next() {
		var resource Resource
		if err := rows.Scan(&resource.ID, &resource.Title, &resource.URL); err != nil {
			return err
		}
		item.Resources = append(item.Resources, resource)
	}
	return rows.Err()
}

type queryer interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

func queryReferences(ctx context.Context, q queryer, sql, id string) ([]Reference, error) {
	rows, err := q.Query(ctx, sql, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]Reference, 0)
	for rows.Next() {
		var item Reference
		if err := rows.Scan(&item.ID, &item.Name); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func replaceRelations(ctx context.Context, tx pgx.Tx, idpID, taskID string, input Input) error {
	if _, err := tx.Exec(ctx, `DELETE FROM task_competencies WHERE task_id=$1`, taskID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM task_tags WHERE task_id=$1`, taskID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM task_resources WHERE task_id=$1`, taskID); err != nil {
		return err
	}

	for _, competencyID := range unique(input.CompetencyIDs) {
		tag, err := tx.Exec(ctx, `
			INSERT INTO task_competencies (task_id, competency_id)
			SELECT $1, competency_id FROM idp_competencies WHERE idp_id=$2 AND competency_id=$3
		`, taskID, idpID, competencyID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() != 1 {
			return ErrInvalidInput
		}
	}
	for _, tagID := range unique(input.TagIDs) {
		tag, err := tx.Exec(ctx, `
			INSERT INTO task_tags (task_id, tag_id)
			SELECT $1, id FROM tags WHERE id=$2 AND is_active=true
		`, taskID, tagID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() != 1 {
			return ErrInvalidInput
		}
	}
	for _, resource := range input.Resources {
		if _, err := tx.Exec(ctx, `INSERT INTO task_resources (task_id, title, url) VALUES ($1,$2,$3)`,
			taskID, strings.TrimSpace(resource.Title), strings.TrimSpace(resource.URL)); err != nil {
			return err
		}
	}
	return nil
}

func validateCategory(ctx context.Context, tx pgx.Tx, categoryID *string) error {
	if categoryID == nil || strings.TrimSpace(*categoryID) == "" {
		return nil
	}
	var exists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM task_categories WHERE id=$1 AND is_active=true)`, strings.TrimSpace(*categoryID)).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return ErrInvalidInput
	}
	return nil
}

func validateInput(input Input, plan *planAccess) error {
	if strings.TrimSpace(input.Title) == "" || len([]rune(strings.TrimSpace(input.Title))) > 200 ||
		(input.Description != nil && len([]rune(*input.Description)) > 5000) ||
		!oneOf(input.Priority, "low", "medium", "high") || !validStatus(input.Status) ||
		input.Progress < 0 || input.Progress > 100 || !validRating(input.ManagerRating) ||
		(input.Status == "completed" && input.Progress != 100) {
		return ErrInvalidInput
	}
	if input.DueDate != nil && (input.DueDate.Before(plan.StartDate) || input.DueDate.After(plan.EndDate)) {
		return ErrInvalidInput
	}
	for _, resource := range input.Resources {
		parsed, err := url.ParseRequestURI(strings.TrimSpace(resource.URL))
		if strings.TrimSpace(resource.Title) == "" || len([]rune(strings.TrimSpace(resource.Title))) > 200 || err != nil ||
			(parsed.Scheme != "http" && parsed.Scheme != "https") || len(resource.URL) > 1000 {
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
	_, err = tx.Exec(ctx, `INSERT INTO audit_logs (actor_id, entity_type, entity_id, action, old_value, new_value)
		VALUES ($1,'task',$2,$3,$4,$5)`, actorID, entityID, action, oldJSON, newJSON)
	return err
}

func marshalNullable(value any) ([]byte, error) {
	if value == nil {
		return nil, nil
	}
	return json.Marshal(value)
}
func canRead(a idp.Access, p *planAccess) bool {
	return a.IsHR || a.UserID == p.EmployeeID || (a.Manager && a.UserID == p.ManagerID)
}
func canManage(a idp.Access, p *planAccess) bool {
	return a.IsHR || (a.Manager && a.UserID == p.ManagerID)
}
func editablePlan(status string) bool { return status == "draft" || status == "active" }
func validStatus(v string) bool {
	return oneOf(v, "not_started", "in_progress", "completed", "cancelled")
}
func validRating(v *string) bool {
	return v == nil || oneOf(strings.TrimSpace(*v), "met", "partially_met", "not_met")
}

func taskOrderBy(sort, order string) (string, error) {
	expressions := map[string]string{
		"due_date": "t.due_date",
		"priority": "CASE t.priority WHEN 'high' THEN 1 WHEN 'medium' THEN 2 ELSE 3 END",
		"status":   "CASE t.status WHEN 'in_progress' THEN 1 WHEN 'not_started' THEN 2 WHEN 'completed' THEN 3 ELSE 4 END",
	}
	if sort == "" {
		sort = "due_date"
	}
	expression, ok := expressions[sort]
	if !ok {
		return "", ErrInvalidInput
	}
	if order == "" {
		order = "asc"
	}
	if order != "asc" && order != "desc" {
		return "", ErrInvalidInput
	}
	return expression + " " + strings.ToUpper(order) + " NULLS LAST", nil
}

func normalizeNewTask(input Input) Input {
	input.Status = "not_started"
	input.Progress = 0
	input.ManagerRating = nil
	input.ManagerComment = nil
	return input
}

func normalizeManagerUpdate(current *Task, input Input) (Input, error) {
	input.Status = current.Status
	input.Progress = current.Progress
	if current.Status != "completed" && (input.ManagerRating != nil || input.ManagerComment != nil) {
		return Input{}, ErrInvalidInput
	}
	return input, nil
}

func reviewChanged(current *Task, input Input) bool {
	currentRating, nextRating := pointerValue(trimmed(current.ManagerRating)), pointerValue(trimmed(input.ManagerRating))
	currentComment, nextComment := pointerValue(trimmed(current.ManagerComment)), pointerValue(trimmed(input.ManagerComment))
	return (nextRating != "" || nextComment != "") && (currentRating != nextRating || currentComment != nextComment)
}

func taskDefinitionChanged(current *Task, input Input) bool {
	if current.Title != strings.TrimSpace(input.Title) || pointerValue(trimmed(current.Description)) != pointerValue(trimmed(input.Description)) ||
		current.Priority != input.Priority || pointerValue(current.DueDate) != dateValue(input.DueDate) ||
		categoryID(current.Category) != pointerValue(trimmed(input.CategoryID)) {
		return true
	}
	return !slices.Equal(sortedReferenceIDs(current.Competencies), sortedStrings(input.CompetencyIDs)) ||
		!slices.Equal(sortedReferenceIDs(current.Tags), sortedStrings(input.TagIDs)) ||
		!slices.Equal(sortedResources(current.Resources), sortedResources(input.Resources))
}

func categoryID(value *Reference) string {
	if value == nil {
		return ""
	}
	return value.ID
}

func dateValue(value *time.Time) string {
	if value == nil {
		return ""
	}
	return value.Format(time.DateOnly)
}

func sortedReferenceIDs(values []Reference) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		result = append(result, value.ID)
	}
	slices.Sort(result)
	return result
}

func sortedStrings(values []string) []string {
	result := unique(values)
	slices.Sort(result)
	return result
}

func sortedResources(values []Resource) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		result = append(result, strings.TrimSpace(value.Title)+"\x00"+strings.TrimSpace(value.URL))
	}
	slices.Sort(result)
	return result
}

func pointerValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
func oneOf(value string, allowed ...string) bool {
	for _, item := range allowed {
		if value == item {
			return true
		}
	}
	return false
}
func trimmed(value *string) *string {
	if value == nil {
		return nil
	}
	result := strings.TrimSpace(*value)
	if result == "" {
		return nil
	}
	return &result
}
func unique(values []string) []string {
	seen := map[string]bool{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" && !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	return result
}
