package users

import (
	"context"
	"errors"
	"strings"
	"time"

	"idp-platform/backend/internal/auth"
	"idp-platform/backend/internal/notification"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrUserNotFound           = errors.New("user not found")
	ErrEmailExists            = errors.New("email already exists")
	ErrInvalidCurrentPassword = errors.New("current password is invalid")
)

type Service struct {
	db          *pgxpool.Pool
	publisher   notification.Publisher
	frontendURL string
}

type User struct {
	ID             string   `json:"id"`
	Email          string   `json:"email"`
	FirstName      string   `json:"first_name"`
	LastName       string   `json:"last_name"`
	MiddleName     *string  `json:"middle_name,omitempty"`
	Position       string   `json:"position"`
	DepartmentID   *string  `json:"department_id,omitempty"`
	DepartmentName *string  `json:"department_name,omitempty"`
	ManagerID      *string  `json:"manager_id,omitempty"`
	AvatarURL      *string  `json:"avatar_url,omitempty"`
	IsActive       bool     `json:"is_active"`
	Roles          []string `json:"roles"`
}

type IDPSummary struct {
	ID             string  `json:"id"`
	EmployeeID     string  `json:"employee_id"`
	ManagerID      string  `json:"manager_id"`
	Title          string  `json:"title"`
	Goals          *string `json:"goals,omitempty"`
	StartDate      string  `json:"start_date"`
	EndDate        string  `json:"end_date"`
	Status         string  `json:"status"`
	TasksTotal     int     `json:"tasks_total"`
	TasksCompleted int     `json:"tasks_completed"`
	Progress       int     `json:"progress"`
}

type EmployeeProfile struct {
	User           User                `json:"user"`
	ManagerName    *string             `json:"manager_name,omitempty"`
	DepartmentName *string             `json:"department_name,omitempty"`
	IDPs           []IDPSummary        `json:"idps"`
	Progress       []ProgressPoint     `json:"progress"`
	Competencies   []CompetencyProfile `json:"competencies"`
}

type ProgressPoint struct {
	Week     string `json:"week"`
	Progress int    `json:"progress"`
}

type CompetencyProfile struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	CurrentLevel int    `json:"current_level"`
	TargetLevel  int    `json:"target_level"`
}

type ListResult struct {
	Data []User   `json:"data"`
	Meta ListMeta `json:"meta"`
}

type ListMeta struct {
	Total      int `json:"total"`
	Page       int `json:"page"`
	Limit      int `json:"limit"`
	TotalPages int `json:"total_pages"`
}

type ListParams struct {
	Page  int
	Limit int
	Query string
}

type CreateInput struct {
	Email        string
	Password     string
	FirstName    string
	LastName     string
	MiddleName   *string
	Position     string
	DepartmentID *string
	ManagerID    *string
	Roles        []string
}

type UpdateInput struct {
	FirstName    string
	LastName     string
	MiddleName   *string
	Position     string
	DepartmentID *string
	ManagerID    *string
	IsActive     bool
	Roles        []string
}

type UpdateProfileInput struct {
	FirstName  string
	LastName   string
	MiddleName *string
}

type ImportInput struct {
	Rows []CreateInput
}

type ImportResult struct {
	Created int              `json:"created"`
	Failed  int              `json:"failed"`
	Errors  []ImportRowError `json:"errors"`
}

type ImportRowError struct {
	Row     int    `json:"row"`
	Email   string `json:"email,omitempty"`
	Message string `json:"message"`
}

func NewService(db *pgxpool.Pool, publisher notification.Publisher, frontendURL string) *Service {
	return &Service{db: db, publisher: publisher, frontendURL: strings.TrimRight(frontendURL, "/")}
}

func (s *Service) List(ctx context.Context, params ListParams) (*ListResult, error) {
	if params.Page < 1 {
		params.Page = 1
	}
	if params.Limit < 1 || params.Limit > 100 {
		params.Limit = 50
	}

	search := "%" + strings.TrimSpace(params.Query) + "%"
	offset := (params.Page - 1) * params.Limit

	var total int
	if err := s.db.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM users
		WHERE $1 = '%%'
			OR email ILIKE $1
			OR first_name ILIKE $1
			OR last_name ILIKE $1
	`, search).Scan(&total); err != nil {
		return nil, err
	}

	rows, err := s.db.Query(ctx, `
		SELECT u.id::text,u.email,u.first_name,u.last_name,u.middle_name,u.position,u.department_id::text,d.name,u.manager_id::text,u.avatar_url,u.is_active
		FROM users u LEFT JOIN departments d ON d.id=u.department_id
		WHERE $1 = '%%'
			OR email ILIKE $1
			OR first_name ILIKE $1
			OR last_name ILIKE $1
		ORDER BY u.created_at DESC
		LIMIT $2 OFFSET $3
	`, search, params.Limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := &ListResult{
		Data: make([]User, 0, params.Limit),
		Meta: ListMeta{
			Total:      total,
			Page:       params.Page,
			Limit:      params.Limit,
			TotalPages: totalPages(total, params.Limit),
		},
	}

	for rows.Next() {
		var user User
		if err := rows.Scan(&user.ID, &user.Email, &user.FirstName, &user.LastName, &user.MiddleName, &user.Position, &user.DepartmentID, &user.DepartmentName, &user.ManagerID, &user.AvatarURL, &user.IsActive); err != nil {
			return nil, err
		}
		result.Data = append(result.Data, user)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := s.assignRoles(ctx, result.Data); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *Service) Get(ctx context.Context, userID string) (*User, error) {
	var user User
	err := s.db.QueryRow(ctx, `
		SELECT u.id::text,u.email,u.first_name,u.last_name,u.middle_name,u.position,u.department_id::text,d.name,u.manager_id::text,u.avatar_url,u.is_active
		FROM users u LEFT JOIN departments d ON d.id=u.department_id WHERE u.id=$1
	`, userID).Scan(&user.ID, &user.Email, &user.FirstName, &user.LastName, &user.MiddleName, &user.Position, &user.DepartmentID, &user.DepartmentName, &user.ManagerID, &user.AvatarURL, &user.IsActive)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	user.Roles, err = s.roles(ctx, user.ID)
	if err != nil {
		return nil, err
	}

	return &user, nil
}

func (s *Service) ListSubordinates(ctx context.Context, managerID string) ([]User, error) {
	rows, err := s.db.Query(ctx, `
		SELECT u.id::text,u.email,u.first_name,u.last_name,u.middle_name,u.position,u.department_id::text,d.name,u.manager_id::text,u.avatar_url,u.is_active
		FROM users u LEFT JOIN departments d ON d.id=u.department_id WHERE u.manager_id=$1 AND u.is_active=true
		ORDER BY last_name, first_name
	`, managerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]User, 0)
	for rows.Next() {
		var user User
		if err := rows.Scan(&user.ID, &user.Email, &user.FirstName, &user.LastName, &user.MiddleName, &user.Position, &user.DepartmentID, &user.DepartmentName, &user.ManagerID, &user.AvatarURL, &user.IsActive); err != nil {
			return nil, err
		}
		result = append(result, user)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := s.assignRoles(ctx, result); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *Service) ListIDPs(ctx context.Context, userID string) ([]IDPSummary, error) {
	rows, err := s.db.Query(ctx, `
		SELECT i.id::text,
			i.employee_id::text,
			i.manager_id::text,
			i.title,
			i.goals,
			i.start_date,
			i.end_date,
			i.status,
			COUNT(t.id)::int,
			COUNT(t.id) FILTER (WHERE t.status = 'completed')::int,
			COALESCE(ROUND(AVG(t.progress))::int, 0)
		FROM idps i
		LEFT JOIN tasks t ON t.idp_id = i.id AND t.deleted_at IS NULL
		WHERE i.employee_id = $1 AND i.archived_at IS NULL
		GROUP BY i.id
		ORDER BY i.created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]IDPSummary, 0)
	for rows.Next() {
		var item IDPSummary
		var startDate time.Time
		var endDate time.Time
		if err := rows.Scan(
			&item.ID,
			&item.EmployeeID,
			&item.ManagerID,
			&item.Title,
			&item.Goals,
			&startDate,
			&endDate,
			&item.Status,
			&item.TasksTotal,
			&item.TasksCompleted,
			&item.Progress,
		); err != nil {
			return nil, err
		}
		item.StartDate = startDate.Format(time.DateOnly)
		item.EndDate = endDate.Format(time.DateOnly)
		result = append(result, item)
	}

	return result, rows.Err()
}

func (s *Service) EmployeeProfile(ctx context.Context, userID string) (*EmployeeProfile, error) {
	result := &EmployeeProfile{IDPs: []IDPSummary{}, Progress: []ProgressPoint{}, Competencies: []CompetencyProfile{}}
	err := s.db.QueryRow(ctx, `
		SELECT u.id::text,u.email,u.first_name,u.last_name,u.middle_name,u.position,u.department_id::text,d.name,u.manager_id::text,u.avatar_url,u.is_active,
			NULLIF(concat_ws(' ',m.last_name,m.first_name,m.middle_name),''),d.name
		FROM users u LEFT JOIN users m ON m.id=u.manager_id LEFT JOIN departments d ON d.id=u.department_id
		WHERE u.id=$1
	`, userID).Scan(&result.User.ID, &result.User.Email, &result.User.FirstName, &result.User.LastName, &result.User.MiddleName,
		&result.User.Position, &result.User.DepartmentID, &result.User.DepartmentName, &result.User.ManagerID, &result.User.AvatarURL, &result.User.IsActive, &result.ManagerName, &result.DepartmentName)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}
	result.User.Roles, err = s.roles(ctx, userID)
	if err != nil {
		return nil, err
	}
	result.IDPs, err = s.ListIDPs(ctx, userID)
	if err != nil {
		return nil, err
	}
	rows, err := s.db.Query(ctx, `
		WITH weeks AS (
			SELECT generate_series(date_trunc('week',NOW()-INTERVAL '6 months'),date_trunc('week',NOW()),INTERVAL '1 week') week
		)
		SELECT w.week::date,COALESCE(ROUND(AVG(CASE WHEN t.created_at<w.week+INTERVAL '1 week' THEN
			COALESCE((SELECT (a.new_value->>'progress')::int FROM audit_logs a
				WHERE a.entity_type='task' AND a.entity_id=t.id AND a.action='task.progress_changed'
					AND a.created_at<w.week+INTERVAL '1 week' ORDER BY a.created_at DESC LIMIT 1),
				CASE WHEN t.updated_at<w.week+INTERVAL '1 week' THEN t.progress ELSE 0 END)
		END))::int,0)
		FROM weeks w LEFT JOIN tasks t ON t.idp_id IN(SELECT id FROM idps WHERE employee_id=$1) AND t.deleted_at IS NULL
		GROUP BY w.week ORDER BY w.week
	`, userID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var point ProgressPoint
		var week time.Time
		if err := rows.Scan(&week, &point.Progress); err != nil {
			rows.Close()
			return nil, err
		}
		point.Week = week.Format(time.DateOnly)
		result.Progress = append(result.Progress, point)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	competencyRows, err := s.db.Query(ctx, `
		SELECT c.id::text,c.name,COALESCE(MAX(ic.current_level),0)::int,MAX(ic.target_level)::int
		FROM idp_competencies ic JOIN competencies c ON c.id=ic.competency_id
		JOIN idps i ON i.id=ic.idp_id WHERE i.employee_id=$1
		GROUP BY c.id ORDER BY c.name
	`, userID)
	if err != nil {
		return nil, err
	}
	defer competencyRows.Close()
	for competencyRows.Next() {
		var item CompetencyProfile
		if err := competencyRows.Scan(&item.ID, &item.Name, &item.CurrentLevel, &item.TargetLevel); err != nil {
			return nil, err
		}
		result.Competencies = append(result.Competencies, item)
	}
	return result, competencyRows.Err()
}

func (s *Service) IsDirectManager(ctx context.Context, managerID, employeeID string) (bool, error) {
	var exists bool
	err := s.db.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM users
			WHERE id = $1 AND manager_id = $2 AND is_active = true
		)
	`, employeeID, managerID).Scan(&exists)
	return exists, err
}

func (s *Service) Create(ctx context.Context, input CreateInput) (*User, error) {
	passwordHash, err := auth.HashPassword(input.Password)
	if err != nil {
		return nil, err
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var userID string
	err = tx.QueryRow(ctx, `
		INSERT INTO users (email,password_hash,first_name,last_name,middle_name,position,department_id,manager_id)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		RETURNING id::text
	`, strings.TrimSpace(input.Email), passwordHash, input.FirstName, input.LastName, input.MiddleName, input.Position, input.DepartmentID, input.ManagerID).Scan(&userID)
	if err != nil {
		if strings.Contains(err.Error(), "users_email_key") {
			return nil, ErrEmailExists
		}
		return nil, err
	}

	if err := replaceRoles(ctx, tx, userID, normalizeRoles(input.Roles)); err != nil {
		return nil, err
	}
	if s.publisher != nil {
		if err := s.publisher.EnqueueTx(ctx, tx, notification.Job{
			UserID: userID, To: []string{strings.TrimSpace(input.Email)}, Template: notification.WelcomeTemplate,
			Data: map[string]string{"first_name": input.FirstName, "login_url": s.frontendURL},
		}); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return s.Get(ctx, userID)
}

func (s *Service) Import(ctx context.Context, input ImportInput) ImportResult {
	result := ImportResult{Errors: make([]ImportRowError, 0)}

	for index, row := range input.Rows {
		rowNumber := index + 2
		if _, err := s.Create(ctx, row); err != nil {
			result.Failed++
			result.Errors = append(result.Errors, ImportRowError{
				Row:     rowNumber,
				Email:   strings.TrimSpace(row.Email),
				Message: importErrorMessage(err),
			})
			continue
		}

		result.Created++
	}

	return result
}

func importErrorMessage(err error) string {
	switch {
	case errors.Is(err, ErrEmailExists):
		return "email already exists"
	case errors.Is(err, auth.ErrWeakPassword):
		return auth.ErrWeakPassword.Error()
	default:
		return "user was not created"
	}
}

func (s *Service) Update(ctx context.Context, userID string, input UpdateInput) (*User, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	tag, err := tx.Exec(ctx, `
		UPDATE users
		SET first_name = $2,
			last_name = $3,
			middle_name = $4,
			position = $5,
			department_id = $6,
			manager_id = $7,
			is_active = $8,
			updated_at = NOW()
		WHERE id = $1
	`, userID, input.FirstName, input.LastName, input.MiddleName, input.Position, input.DepartmentID, input.ManagerID, input.IsActive)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		return nil, ErrUserNotFound
	}

	if err := replaceRoles(ctx, tx, userID, normalizeRoles(input.Roles)); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `UPDATE refresh_tokens SET revoked_at=NOW() WHERE user_id=$1 AND revoked_at IS NULL`, userID); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `UPDATE idps SET manager_id=$2, updated_at=NOW() WHERE employee_id=$1 AND manager_id IS DISTINCT FROM $2 AND archived_at IS NULL`, userID, input.ManagerID); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return s.Get(ctx, userID)
}

func (s *Service) UpdateProfile(ctx context.Context, userID string, input UpdateProfileInput) (*User, error) {
	tag, err := s.db.Exec(ctx, `
		UPDATE users
		SET first_name = $2,
			last_name = $3,
			middle_name = $4,
			updated_at = NOW()
		WHERE id = $1 AND is_active = true
	`, userID, input.FirstName, input.LastName, input.MiddleName)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		return nil, ErrUserNotFound
	}

	return s.Get(ctx, userID)
}

func (s *Service) ChangePassword(ctx context.Context, userID, currentPassword, newPassword string) error {
	var currentHash string
	err := s.db.QueryRow(ctx, `
		SELECT password_hash
		FROM users
		WHERE id = $1 AND is_active = true
	`, userID).Scan(&currentHash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrUserNotFound
		}
		return err
	}

	if !auth.ComparePassword(currentHash, currentPassword) {
		return ErrInvalidCurrentPassword
	}

	newHash, err := auth.HashPassword(newPassword)
	if err != nil {
		return err
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	_, err = tx.Exec(ctx, `
		UPDATE users
		SET password_hash = $2,
			failed_login_attempts = 0,
			locked_until = NULL,
			updated_at = NOW()
		WHERE id = $1
	`, userID, newHash)
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE refresh_tokens SET revoked_at=NOW() WHERE user_id=$1 AND revoked_at IS NULL`, userID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Service) UpdateAvatar(ctx context.Context, userID, avatarURL string) (*User, error) {
	tag, err := s.db.Exec(ctx, `
		UPDATE users
		SET avatar_url = $2,
			updated_at = NOW()
		WHERE id = $1 AND is_active = true
	`, userID, avatarURL)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		return nil, ErrUserNotFound
	}

	return s.Get(ctx, userID)
}

func (s *Service) Deactivate(ctx context.Context, userID string) error {
	tag, err := s.db.Exec(ctx, `
		UPDATE users
		SET is_active = false,
			updated_at = NOW()
		WHERE id = $1
	`, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrUserNotFound
	}
	return nil
}

func (s *Service) Activate(ctx context.Context, userID string) (*User, error) {
	tag, err := s.db.Exec(ctx, `
		UPDATE users
		SET is_active = true,
			failed_login_attempts = 0,
			locked_until = NULL,
			updated_at = NOW()
		WHERE id = $1
	`, userID)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		return nil, ErrUserNotFound
	}

	return s.Get(ctx, userID)
}

func (s *Service) roles(ctx context.Context, userID string) ([]string, error) {
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

	roles := make([]string, 0, 3)
	for rows.Next() {
		var role string
		if err := rows.Scan(&role); err != nil {
			return nil, err
		}
		roles = append(roles, role)
	}

	return roles, rows.Err()
}

func (s *Service) assignRoles(ctx context.Context, users []User) error {
	if len(users) == 0 {
		return nil
	}
	ids := make([]string, 0, len(users))
	byID := make(map[string]*User, len(users))
	for i := range users {
		ids = append(ids, users[i].ID)
		byID[users[i].ID] = &users[i]
		users[i].Roles = []string{}
	}
	rows, err := s.db.Query(ctx, `SELECT user_id::text, role FROM user_roles WHERE user_id = ANY($1::uuid[]) ORDER BY role`, ids)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id, role string
		if err := rows.Scan(&id, &role); err != nil {
			return err
		}
		if user := byID[id]; user != nil {
			user.Roles = append(user.Roles, role)
		}
	}
	return rows.Err()
}

func replaceRoles(ctx context.Context, tx pgx.Tx, userID string, roles []string) error {
	if _, err := tx.Exec(ctx, `DELETE FROM user_roles WHERE user_id = $1`, userID); err != nil {
		return err
	}
	for _, role := range roles {
		if _, err := tx.Exec(ctx, `
			INSERT INTO user_roles (user_id, role)
			VALUES ($1, $2)
		`, userID, role); err != nil {
			return err
		}
	}
	return nil
}

func normalizeRoles(roles []string) []string {
	allowed := map[string]bool{"employee": true, "manager": true, "hr_admin": true}
	seen := map[string]bool{}
	result := []string{"employee"}

	for _, role := range roles {
		role = strings.TrimSpace(role)
		if !allowed[role] || seen[role] || role == "employee" {
			continue
		}
		seen[role] = true
		result = append(result, role)
	}

	return result
}

func totalPages(total, limit int) int {
	if total == 0 {
		return 0
	}
	return (total + limit - 1) / limit
}
