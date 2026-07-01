package templates

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"idp-platform/backend/internal/idp"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound     = errors.New("template not found")
	ErrForbidden    = errors.New("template access forbidden")
	ErrInvalidInput = errors.New("invalid template input")
)

type Service struct{ db *pgxpool.Pool }
type Template struct {
	ID           string       `json:"id"`
	CreatorID    string       `json:"creator_id"`
	Title        string       `json:"title"`
	Description  *string      `json:"description,omitempty"`
	Goals        *string      `json:"goals,omitempty"`
	TargetRole   *string      `json:"target_role,omitempty"`
	IsActive     bool         `json:"is_active"`
	Tasks        []Task       `json:"tasks"`
	Competencies []Competency `json:"competencies"`
	CreatedAt    time.Time    `json:"created_at"`
	UpdatedAt    time.Time    `json:"updated_at"`
}
type Task struct {
	ID            string  `json:"id,omitempty"`
	Title         string  `json:"title"`
	Description   *string `json:"description,omitempty"`
	CategoryID    *string `json:"category_id,omitempty"`
	Priority      string  `json:"priority"`
	DueOffsetDays *int    `json:"due_offset_days,omitempty"`
}
type Competency struct {
	CompetencyID string `json:"competency_id"`
	Name         string `json:"name,omitempty"`
	TargetLevel  int    `json:"target_level"`
}
type Input struct {
	Title        string       `json:"title"`
	Description  *string      `json:"description"`
	Goals        *string      `json:"goals"`
	TargetRole   *string      `json:"target_role"`
	IsActive     bool         `json:"is_active"`
	Tasks        []Task       `json:"tasks"`
	Competencies []Competency `json:"competencies"`
}
type ApplyInput struct {
	EmployeeID string    `json:"employee_id"`
	Title      string    `json:"title"`
	StartDate  time.Time `json:"-"`
	EndDate    time.Time `json:"-"`
}

func NewService(db *pgxpool.Pool) *Service { return &Service{db: db} }

func (s *Service) List(ctx context.Context, access idp.Access) ([]Template, error) {
	if !access.Manager && !access.IsHR {
		return nil, ErrForbidden
	}
	rows, err := s.db.Query(ctx, `SELECT id::text,creator_id::text,title,description,goals,target_role,is_active,created_at,updated_at FROM idp_templates WHERE is_active=true OR creator_id=$1 OR $2 ORDER BY is_active DESC,title`, access.UserID, access.IsHR)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]Template, 0)
	for rows.Next() {
		var x Template
		if err := rows.Scan(&x.ID, &x.CreatorID, &x.Title, &x.Description, &x.Goals, &x.TargetRole, &x.IsActive, &x.CreatedAt, &x.UpdatedAt); err != nil {
			return nil, err
		}
		if err = s.load(ctx, &x); err != nil {
			return nil, err
		}
		result = append(result, x)
	}
	return result, rows.Err()
}

func (s *Service) Get(ctx context.Context, access idp.Access, id string) (*Template, error) {
	var x Template
	err := s.db.QueryRow(ctx, `SELECT id::text,creator_id::text,title,description,goals,target_role,is_active,created_at,updated_at FROM idp_templates WHERE id=$1`, id).Scan(&x.ID, &x.CreatorID, &x.Title, &x.Description, &x.Goals, &x.TargetRole, &x.IsActive, &x.CreatedAt, &x.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if !access.IsHR && !access.Manager {
		return nil, ErrForbidden
	}
	if err = s.load(ctx, &x); err != nil {
		return nil, err
	}
	return &x, nil
}

func (s *Service) Create(ctx context.Context, access idp.Access, in Input) (*Template, error) {
	if !access.Manager && !access.IsHR {
		return nil, ErrForbidden
	}
	if !valid(in) {
		return nil, ErrInvalidInput
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	var id string
	err = tx.QueryRow(ctx, `INSERT INTO idp_templates(creator_id,title,description,goals,target_role,is_active) VALUES($1,$2,$3,$4,$5,$6) RETURNING id::text`, access.UserID, strings.TrimSpace(in.Title), trim(in.Description), trim(in.Goals), trim(in.TargetRole), in.IsActive).Scan(&id)
	if err != nil {
		return nil, err
	}
	if err = replace(ctx, tx, id, in); err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.Get(ctx, access, id)
}

func (s *Service) Update(ctx context.Context, access idp.Access, id string, in Input) (*Template, error) {
	current, err := s.Get(ctx, access, id)
	if err != nil {
		return nil, err
	}
	if !access.IsHR && current.CreatorID != access.UserID {
		return nil, ErrForbidden
	}
	if !valid(in) {
		return nil, ErrInvalidInput
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	_, err = tx.Exec(ctx, `UPDATE idp_templates SET title=$2,description=$3,goals=$4,target_role=$5,is_active=$6,updated_at=NOW() WHERE id=$1`, id, strings.TrimSpace(in.Title), trim(in.Description), trim(in.Goals), trim(in.TargetRole), in.IsActive)
	if err != nil {
		return nil, err
	}
	if err = replace(ctx, tx, id, in); err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.Get(ctx, access, id)
}

func (s *Service) Archive(ctx context.Context, access idp.Access, id string) error {
	current, err := s.Get(ctx, access, id)
	if err != nil {
		return err
	}
	if !access.IsHR && current.CreatorID != access.UserID {
		return ErrForbidden
	}
	_, err = s.db.Exec(ctx, `UPDATE idp_templates SET is_active=false,updated_at=NOW() WHERE id=$1`, id)
	return err
}

func (s *Service) Apply(ctx context.Context, access idp.Access, templateID string, in ApplyInput) (string, error) {
	if !access.Manager && !access.IsHR {
		return "", ErrForbidden
	}
	tpl, err := s.Get(ctx, access, templateID)
	if err != nil {
		return "", err
	}
	if !tpl.IsActive || in.EmployeeID == "" || strings.TrimSpace(in.Title) == "" || in.EndDate.Before(in.StartDate) {
		return "", ErrInvalidInput
	}
	days := int(in.EndDate.Sub(in.StartDate).Hours() / 24)
	for _, t := range tpl.Tasks {
		if t.DueOffsetDays != nil && *t.DueOffsetDays > days {
			return "", ErrInvalidInput
		}
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)
	var managerID string
	var active bool
	err = tx.QueryRow(ctx, `SELECT manager_id::text,is_active FROM users WHERE id=$1`, in.EmployeeID).Scan(&managerID, &active)
	if err != nil {
		return "", fmt.Errorf("load employee manager: %w", err)
	}
	if !active || managerID == "" {
		return "", ErrInvalidInput
	}
	if !access.IsHR && managerID != access.UserID {
		return "", ErrForbidden
	}
	var planID string
	err = tx.QueryRow(ctx, `INSERT INTO idps(employee_id,manager_id,title,goals,start_date,end_date) VALUES($1,$2,$3,$4,$5,$6) RETURNING id::text`, in.EmployeeID, managerID, strings.TrimSpace(in.Title), tpl.Goals, in.StartDate, in.EndDate).Scan(&planID)
	if err != nil {
		return "", fmt.Errorf("create plan: %w", err)
	}
	for _, c := range tpl.Competencies {
		if _, err = tx.Exec(ctx, `INSERT INTO idp_competencies(idp_id,competency_id,target_level) VALUES($1,$2,$3)`, planID, c.CompetencyID, c.TargetLevel); err != nil {
			return "", fmt.Errorf("copy competency: %w", err)
		}
	}
	for _, t := range tpl.Tasks {
		var due *time.Time
		if t.DueOffsetDays != nil {
			x := in.StartDate.AddDate(0, 0, *t.DueOffsetDays)
			due = &x
		}
		if _, err = tx.Exec(ctx, `INSERT INTO tasks(idp_id,title,description,category_id,priority,due_date) VALUES($1,$2,$3,$4,$5,$6)`, planID, t.Title, t.Description, t.CategoryID, t.Priority, due); err != nil {
			return "", fmt.Errorf("copy task: %w", err)
		}
	}
	if _, err = tx.Exec(ctx, `INSERT INTO audit_logs(actor_id,entity_type,entity_id,action,new_value) VALUES($1,'idp',$2,'idp.created_from_template',jsonb_build_object('template_id',$3::text))`, access.UserID, planID, templateID); err != nil {
		return "", fmt.Errorf("write audit: %w", err)
	}
	if err = tx.Commit(ctx); err != nil {
		return "", err
	}
	return planID, nil
}

func (s *Service) load(ctx context.Context, x *Template) error {
	x.Tasks = []Task{}
	rows, err := s.db.Query(ctx, `SELECT id::text,title,description,category_id::text,priority,due_offset_days FROM template_tasks WHERE template_id=$1 ORDER BY id`, x.ID)
	if err != nil {
		return err
	}
	for rows.Next() {
		var t Task
		if err = rows.Scan(&t.ID, &t.Title, &t.Description, &t.CategoryID, &t.Priority, &t.DueOffsetDays); err != nil {
			rows.Close()
			return err
		}
		x.Tasks = append(x.Tasks, t)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	x.Competencies = []Competency{}
	cr, err := s.db.Query(ctx, `SELECT tc.competency_id::text,c.name,tc.target_level FROM template_competencies tc JOIN competencies c ON c.id=tc.competency_id WHERE tc.template_id=$1 ORDER BY c.name`, x.ID)
	if err != nil {
		return err
	}
	defer cr.Close()
	for cr.Next() {
		var c Competency
		if err = cr.Scan(&c.CompetencyID, &c.Name, &c.TargetLevel); err != nil {
			return err
		}
		x.Competencies = append(x.Competencies, c)
	}
	return cr.Err()
}
func replace(ctx context.Context, tx pgx.Tx, id string, in Input) error {
	if _, err := tx.Exec(ctx, `DELETE FROM template_tasks WHERE template_id=$1`, id); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM template_competencies WHERE template_id=$1`, id); err != nil {
		return err
	}
	for _, t := range in.Tasks {
		if _, err := tx.Exec(ctx, `INSERT INTO template_tasks(template_id,title,description,category_id,priority,due_offset_days) VALUES($1,$2,$3,$4,$5,$6)`, id, strings.TrimSpace(t.Title), trim(t.Description), trim(t.CategoryID), t.Priority, t.DueOffsetDays); err != nil {
			return err
		}
	}
	for _, c := range in.Competencies {
		if _, err := tx.Exec(ctx, `INSERT INTO template_competencies(template_id,competency_id,target_level) VALUES($1,$2,$3)`, id, c.CompetencyID, c.TargetLevel); err != nil {
			return err
		}
	}
	return nil
}
func valid(in Input) bool {
	if strings.TrimSpace(in.Title) == "" || len(in.Tasks) == 0 {
		return false
	}
	for _, t := range in.Tasks {
		if strings.TrimSpace(t.Title) == "" || (t.Priority != "low" && t.Priority != "medium" && t.Priority != "high") || (t.DueOffsetDays != nil && (*t.DueOffsetDays < 0 || *t.DueOffsetDays > 3650)) {
			return false
		}
	}
	for _, c := range in.Competencies {
		if c.CompetencyID == "" || c.TargetLevel < 1 || c.TargetLevel > 4 {
			return false
		}
	}
	return true
}
func trim(v *string) *string {
	if v == nil {
		return nil
	}
	x := strings.TrimSpace(*v)
	if x == "" {
		return nil
	}
	return &x
}
