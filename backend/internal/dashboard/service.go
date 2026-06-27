package dashboard

import (
	"context"
	"time"

	"idp-platform/backend/internal/idp"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct{ db *pgxpool.Pool }

type Response struct {
	ActivePlans   []PlanSummary      `json:"active_plans"`
	UpcomingTasks []TaskSummary      `json:"upcoming_tasks"`
	OverdueTasks  []TaskSummary      `json:"overdue_tasks"`
	Competencies  []Competency       `json:"competencies"`
	Activities    []Activity         `json:"activities"`
	Management    *ManagementSummary `json:"management,omitempty"`
}

type PlanSummary struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	EndDate  string `json:"end_date"`
	Progress int    `json:"progress"`
}

type TaskSummary struct {
	ID       string `json:"id"`
	IDPID    string `json:"idp_id"`
	Title    string `json:"title"`
	DueDate  string `json:"due_date"`
	Priority string `json:"priority"`
	Status   string `json:"status"`
	Progress int    `json:"progress"`
}
type Competency struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	CurrentLevel *int   `json:"current_level,omitempty"`
	TargetLevel  int    `json:"target_level"`
}
type Activity struct {
	Action    string    `json:"action"`
	ActorName string    `json:"actor_name"`
	CreatedAt time.Time `json:"created_at"`
}
type ManagementSummary struct {
	Team            []TeamMember    `json:"team"`
	Attention       []AttentionItem `json:"attention"`
	UpcomingEndings []PlanSummary   `json:"upcoming_endings"`
	ActivePlans     int             `json:"active_plans"`
	AverageProgress int             `json:"average_progress"`
	TaskStatuses    map[string]int  `json:"task_statuses"`
}
type TeamMember struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Position  string  `json:"position"`
	AvatarURL *string `json:"avatar_url,omitempty"`
	PlanID    *string `json:"plan_id,omitempty"`
	Progress  int     `json:"progress"`
}
type AttentionItem struct {
	EmployeeID   string `json:"employee_id"`
	EmployeeName string `json:"employee_name"`
	Reason       string `json:"reason"`
	Count        int    `json:"count"`
}

func NewService(db *pgxpool.Pool) *Service { return &Service{db: db} }

func (s *Service) Get(ctx context.Context, access idp.Access) (*Response, error) {
	result := &Response{}
	var err error
	if result.ActivePlans, err = s.plans(ctx, access.UserID); err != nil {
		return nil, err
	}
	if result.UpcomingTasks, err = s.tasks(ctx, access.UserID, false); err != nil {
		return nil, err
	}
	if result.OverdueTasks, err = s.tasks(ctx, access.UserID, true); err != nil {
		return nil, err
	}
	if result.Competencies, err = s.competencies(ctx, access.UserID); err != nil {
		return nil, err
	}
	if result.Activities, err = s.activities(ctx, access.UserID); err != nil {
		return nil, err
	}
	if access.Manager || access.IsHR {
		result.Management, err = s.management(ctx, access)
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (s *Service) plans(ctx context.Context, userID string) ([]PlanSummary, error) {
	rows, err := s.db.Query(ctx, `SELECT i.id::text,i.title,i.end_date,COALESCE(ROUND(AVG(t.progress))::int,0)
		FROM idps i LEFT JOIN tasks t ON t.idp_id=i.id AND t.deleted_at IS NULL
		WHERE i.employee_id=$1 AND i.status='active' AND i.archived_at IS NULL GROUP BY i.id ORDER BY i.end_date`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]PlanSummary, 0)
	for rows.Next() {
		var x PlanSummary
		var date time.Time
		if err := rows.Scan(&x.ID, &x.Title, &date, &x.Progress); err != nil {
			return nil, err
		}
		x.EndDate = date.Format(time.DateOnly)
		items = append(items, x)
	}
	return items, rows.Err()
}

func (s *Service) tasks(ctx context.Context, userID string, overdue bool) ([]TaskSummary, error) {
	rows, err := s.db.Query(ctx, `SELECT t.id::text,t.idp_id::text,t.title,t.due_date,t.priority,t.status,t.progress
		FROM tasks t JOIN idps i ON i.id=t.idp_id WHERE i.employee_id=$1 AND i.status='active' AND i.archived_at IS NULL
		AND t.deleted_at IS NULL AND t.status NOT IN ('completed','cancelled') AND t.due_date IS NOT NULL
		AND (($2 AND t.due_date<CURRENT_DATE) OR (NOT $2 AND t.due_date BETWEEN CURRENT_DATE AND CURRENT_DATE+7))
		ORDER BY t.due_date,CASE t.priority WHEN 'high' THEN 1 WHEN 'medium' THEN 2 ELSE 3 END LIMIT 20`, userID, overdue)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]TaskSummary, 0)
	for rows.Next() {
		var x TaskSummary
		var date time.Time
		if err := rows.Scan(&x.ID, &x.IDPID, &x.Title, &date, &x.Priority, &x.Status, &x.Progress); err != nil {
			return nil, err
		}
		x.DueDate = date.Format(time.DateOnly)
		items = append(items, x)
	}
	return items, rows.Err()
}

func (s *Service) competencies(ctx context.Context, userID string) ([]Competency, error) {
	rows, err := s.db.Query(ctx, `SELECT c.id::text,c.name,MAX(ic.current_level),MAX(ic.target_level) FROM idp_competencies ic JOIN competencies c ON c.id=ic.competency_id JOIN idps i ON i.id=ic.idp_id WHERE i.employee_id=$1 AND i.status='active' AND i.archived_at IS NULL GROUP BY c.id ORDER BY c.name`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Competency, 0)
	for rows.Next() {
		var x Competency
		if err := rows.Scan(&x.ID, &x.Name, &x.CurrentLevel, &x.TargetLevel); err != nil {
			return nil, err
		}
		items = append(items, x)
	}
	return items, rows.Err()
}

func (s *Service) activities(ctx context.Context, userID string) ([]Activity, error) {
	rows, err := s.db.Query(ctx, `SELECT action,actor_name,created_at FROM (
		SELECT a.action,COALESCE(concat_ws(' ',u.last_name,u.first_name),'Система') actor_name,a.created_at FROM audit_logs a LEFT JOIN users u ON u.id=a.actor_id JOIN idps i ON a.entity_type='idp' AND a.entity_id=i.id WHERE i.employee_id=$1
		UNION ALL SELECT a.action,COALESCE(concat_ws(' ',u.last_name,u.first_name),'Система'),a.created_at FROM audit_logs a LEFT JOIN users u ON u.id=a.actor_id JOIN tasks t ON a.entity_type='task' AND a.entity_id=t.id JOIN idps i ON i.id=t.idp_id WHERE i.employee_id=$1
		UNION ALL SELECT 'comment.created',concat_ws(' ',u.last_name,u.first_name),c.created_at FROM comments c JOIN users u ON u.id=c.author_id WHERE c.is_deleted=false AND ((c.entity_type='idp' AND c.entity_id IN(SELECT id FROM idps WHERE employee_id=$1)) OR (c.entity_type='task' AND c.entity_id IN(SELECT t.id FROM tasks t JOIN idps i ON i.id=t.idp_id WHERE i.employee_id=$1)))
	) events ORDER BY created_at DESC LIMIT 10`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Activity, 0)
	for rows.Next() {
		var x Activity
		if err := rows.Scan(&x.Action, &x.ActorName, &x.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, x)
	}
	return items, rows.Err()
}

func (s *Service) management(ctx context.Context, access idp.Access) (*ManagementSummary, error) {
	result := &ManagementSummary{Team: make([]TeamMember, 0), Attention: make([]AttentionItem, 0), UpcomingEndings: make([]PlanSummary, 0), TaskStatuses: map[string]int{"not_started": 0, "in_progress": 0, "completed": 0, "cancelled": 0}}
	rows, err := s.db.Query(ctx, `SELECT u.id::text,concat_ws(' ',u.last_name,u.first_name,u.middle_name),u.position,u.avatar_url,i.id::text,COALESCE(ROUND(AVG(t.progress))::int,0)
		FROM users u LEFT JOIN idps i ON i.employee_id=u.id AND i.status='active' AND i.archived_at IS NULL LEFT JOIN tasks t ON t.idp_id=i.id AND t.deleted_at IS NULL
		WHERE u.is_active=true AND ($1 OR u.manager_id=$2) AND u.id<>$2 GROUP BY u.id,i.id ORDER BY u.last_name,u.first_name`, access.IsHR, access.UserID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var x TeamMember
		if err := rows.Scan(&x.ID, &x.Name, &x.Position, &x.AvatarURL, &x.PlanID, &x.Progress); err != nil {
			rows.Close()
			return nil, err
		}
		result.Team = append(result.Team, x)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	if err := s.db.QueryRow(ctx, `SELECT COUNT(DISTINCT i.id)::int,COALESCE(ROUND(AVG(task_progress))::int,0) FROM idps i LEFT JOIN (SELECT idp_id,AVG(progress) task_progress FROM tasks WHERE deleted_at IS NULL GROUP BY idp_id)t ON t.idp_id=i.id WHERE i.status='active' AND i.archived_at IS NULL AND ($1 OR i.manager_id=$2)`, access.IsHR, access.UserID).Scan(&result.ActivePlans, &result.AverageProgress); err != nil {
		return nil, err
	}
	statusRows, err := s.db.Query(ctx, `SELECT t.status,COUNT(*)::int FROM tasks t JOIN idps i ON i.id=t.idp_id WHERE t.deleted_at IS NULL AND i.status='active' AND i.archived_at IS NULL AND ($1 OR i.manager_id=$2) GROUP BY t.status`, access.IsHR, access.UserID)
	if err != nil {
		return nil, err
	}
	for statusRows.Next() {
		var status string
		var count int
		if err := statusRows.Scan(&status, &count); err != nil {
			statusRows.Close()
			return nil, err
		}
		result.TaskStatuses[status] = count
	}
	statusRows.Close()
	attentionRows, err := s.db.Query(ctx, `SELECT u.id::text,concat_ws(' ',u.last_name,u.first_name),COUNT(t.id)::int FROM users u JOIN idps i ON i.employee_id=u.id AND i.status='active' JOIN tasks t ON t.idp_id=i.id AND t.deleted_at IS NULL WHERE t.due_date<CURRENT_DATE AND t.status NOT IN('completed','cancelled') AND ($1 OR i.manager_id=$2) GROUP BY u.id ORDER BY COUNT(t.id) DESC`, access.IsHR, access.UserID)
	if err != nil {
		return nil, err
	}
	for attentionRows.Next() {
		var x AttentionItem
		x.Reason = "Просроченные задачи"
		if err := attentionRows.Scan(&x.EmployeeID, &x.EmployeeName, &x.Count); err != nil {
			attentionRows.Close()
			return nil, err
		}
		result.Attention = append(result.Attention, x)
	}
	attentionRows.Close()
	endingRows, err := s.db.Query(ctx, `SELECT i.id::text,i.title,i.end_date,COALESCE(ROUND(AVG(t.progress))::int,0) FROM idps i LEFT JOIN tasks t ON t.idp_id=i.id AND t.deleted_at IS NULL WHERE i.status='active' AND i.archived_at IS NULL AND i.end_date BETWEEN CURRENT_DATE AND CURRENT_DATE+30 AND ($1 OR i.manager_id=$2) GROUP BY i.id ORDER BY i.end_date`, access.IsHR, access.UserID)
	if err != nil {
		return nil, err
	}
	for endingRows.Next() {
		var x PlanSummary
		var date time.Time
		if err := endingRows.Scan(&x.ID, &x.Title, &date, &x.Progress); err != nil {
			endingRows.Close()
			return nil, err
		}
		x.EndDate = date.Format(time.DateOnly)
		result.UpcomingEndings = append(result.UpcomingEndings, x)
	}
	err = endingRows.Err()
	endingRows.Close()
	return result, err
}
