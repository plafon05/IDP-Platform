package analytics

import (
	"context"
	"errors"
	"time"

	"idp-platform/backend/internal/idp"

	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrForbidden = errors.New("analytics access forbidden")

// Условие видимости и фильтрации ИПР меняется в одном месте.
const overviewIDPFilterClause = `
		i.archived_at IS NULL AND ($1 OR i.manager_id=$2)
		AND i.start_date<=$4 AND i.end_date>=$3 AND ($5='' OR i.status=$5)`

type Service struct{ db *pgxpool.Pool }

type Filters struct {
	From   time.Time
	To     time.Time
	Status string
}

type Response struct {
	Summary      Summary          `json:"summary"`
	Statuses     []NamedMetric    `json:"statuses"`
	Activity     []WeeklyActivity `json:"activity"`
	Competencies []NamedMetric    `json:"competencies"`
	Categories   []NamedMetric    `json:"categories"`
	Employees    []EmployeeMetric `json:"employees"`
}

type Summary struct {
	Plans           int `json:"plans"`
	Employees       int `json:"employees"`
	Tasks           int `json:"tasks"`
	AverageProgress int `json:"average_progress"`
}

type NamedMetric struct {
	Name  string `json:"name"`
	Value int    `json:"value"`
}

type WeeklyActivity struct {
	Week  string `json:"week"`
	Value int    `json:"value"`
}

type EmployeeMetric struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Position        string `json:"position"`
	Plans           int    `json:"plans"`
	Tasks           int    `json:"tasks"`
	AverageProgress int    `json:"average_progress"`
}

func NewService(db *pgxpool.Pool) *Service { return &Service{db: db} }

func (s *Service) Overview(ctx context.Context, access idp.Access, filters Filters) (*Response, error) {
	if !access.IsHR && !access.Manager {
		return nil, ErrForbidden
	}
	result := &Response{
		Statuses: make([]NamedMetric, 0), Activity: make([]WeeklyActivity, 0),
		Competencies: make([]NamedMetric, 0), Categories: make([]NamedMetric, 0), Employees: make([]EmployeeMetric, 0),
	}
	if err := s.db.QueryRow(ctx, `
		SELECT COUNT(DISTINCT i.id)::int,COUNT(DISTINCT i.employee_id)::int,COUNT(DISTINCT t.id)::int,
			COALESCE(ROUND(AVG(t.progress))::int,0)
		FROM idps i LEFT JOIN tasks t ON t.idp_id=i.id AND t.deleted_at IS NULL
		WHERE `+overviewIDPFilterClause, access.IsHR, access.UserID, filters.From, filters.To, filters.Status).Scan(
		&result.Summary.Plans, &result.Summary.Employees, &result.Summary.Tasks, &result.Summary.AverageProgress,
	); err != nil {
		return nil, err
	}

	rows, err := s.db.Query(ctx, `
		SELECT i.status,COUNT(*)::int FROM idps i
		WHERE `+overviewIDPFilterClause+`
		GROUP BY i.status ORDER BY i.status
	`, access.IsHR, access.UserID, filters.From, filters.To, filters.Status)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var item NamedMetric
		if err := rows.Scan(&item.Name, &item.Value); err != nil {
			rows.Close()
			return nil, err
		}
		result.Statuses = append(result.Statuses, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	activityRows, err := s.db.Query(ctx, `
		SELECT date_trunc('week',a.created_at)::date,COUNT(*)::int
		FROM audit_logs a JOIN tasks t ON a.entity_type='task' AND a.entity_id=t.id
		JOIN idps i ON i.id=t.idp_id
		WHERE a.action='task.progress_changed' AND i.archived_at IS NULL AND ($1 OR i.manager_id=$2)
			AND a.created_at::date BETWEEN $3 AND $4 AND ($5='' OR i.status=$5)
		GROUP BY 1 ORDER BY 1
	`, access.IsHR, access.UserID, filters.From, filters.To, filters.Status)
	if err != nil {
		return nil, err
	}
	for activityRows.Next() {
		var date time.Time
		var item WeeklyActivity
		if err := activityRows.Scan(&date, &item.Value); err != nil {
			activityRows.Close()
			return nil, err
		}
		item.Week = date.Format(time.DateOnly)
		result.Activity = append(result.Activity, item)
	}
	if err := activityRows.Err(); err != nil {
		activityRows.Close()
		return nil, err
	}
	activityRows.Close()

	result.Competencies, err = s.namedMetrics(ctx, `
		SELECT c.name,COUNT(*)::int FROM idp_competencies ic
		JOIN competencies c ON c.id=ic.competency_id JOIN idps i ON i.id=ic.idp_id
		WHERE `+overviewIDPFilterClause+`
		GROUP BY c.id ORDER BY COUNT(*) DESC,c.name LIMIT 10
	`, access, filters)
	if err != nil {
		return nil, err
	}
	result.Categories, err = s.namedMetrics(ctx, `
		SELECT COALESCE(c.name,'Без категории'),COUNT(*)::int FROM tasks t
		JOIN idps i ON i.id=t.idp_id LEFT JOIN task_categories c ON c.id=t.category_id
		WHERE t.deleted_at IS NULL AND `+overviewIDPFilterClause+`
		GROUP BY c.id,c.name ORDER BY COUNT(*) DESC,COALESCE(c.name,'Без категории')
	`, access, filters)
	if err != nil {
		return nil, err
	}

	employeeRows, err := s.db.Query(ctx, `
		SELECT u.id::text,concat_ws(' ',u.last_name,u.first_name,u.middle_name),u.position,
			COUNT(DISTINCT i.id)::int,COUNT(DISTINCT t.id)::int,COALESCE(ROUND(AVG(t.progress))::int,0)
		FROM users u JOIN idps i ON i.employee_id=u.id
		LEFT JOIN tasks t ON t.idp_id=i.id AND t.deleted_at IS NULL
		WHERE u.is_active=true AND `+overviewIDPFilterClause+`
		GROUP BY u.id ORDER BY AVG(t.progress) DESC NULLS LAST,u.last_name,u.first_name
	`, access.IsHR, access.UserID, filters.From, filters.To, filters.Status)
	if err != nil {
		return nil, err
	}
	defer employeeRows.Close()
	for employeeRows.Next() {
		var item EmployeeMetric
		if err := employeeRows.Scan(&item.ID, &item.Name, &item.Position, &item.Plans, &item.Tasks, &item.AverageProgress); err != nil {
			return nil, err
		}
		result.Employees = append(result.Employees, item)
	}
	return result, employeeRows.Err()
}

func (s *Service) namedMetrics(ctx context.Context, query string, access idp.Access, filters Filters) ([]NamedMetric, error) {
	rows, err := s.db.Query(ctx, query, access.IsHR, access.UserID, filters.From, filters.To, filters.Status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]NamedMetric, 0)
	for rows.Next() {
		var item NamedMetric
		if err := rows.Scan(&item.Name, &item.Value); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}
