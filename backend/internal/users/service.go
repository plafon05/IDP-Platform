package users

import (
	"context"
	"errors"
	"strings"
	"time"

	"idp-platform/backend/internal/auth"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrUserNotFound           = errors.New("user not found")
	ErrEmailExists            = errors.New("email already exists")
	ErrInvalidCurrentPassword = errors.New("current password is invalid")
)

type Service struct {
	db *pgxpool.Pool
}

type User struct {
	ID         string   `json:"id"`
	Email      string   `json:"email"`
	FirstName  string   `json:"first_name"`
	LastName   string   `json:"last_name"`
	MiddleName *string  `json:"middle_name,omitempty"`
	Position   string   `json:"position"`
	ManagerID  *string  `json:"manager_id,omitempty"`
	IsActive   bool     `json:"is_active"`
	Roles      []string `json:"roles"`
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
	Email      string
	Password   string
	FirstName  string
	LastName   string
	MiddleName *string
	Position   string
	ManagerID  *string
	Roles      []string
}

type UpdateInput struct {
	FirstName  string
	LastName   string
	MiddleName *string
	Position   string
	ManagerID  *string
	IsActive   bool
	Roles      []string
}

type UpdateProfileInput struct {
	FirstName  string
	LastName   string
	MiddleName *string
	Position   string
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

func NewService(db *pgxpool.Pool) *Service {
	return &Service{db: db}
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
		SELECT id::text, email, first_name, last_name, middle_name, position, manager_id::text, is_active
		FROM users
		WHERE $1 = '%%'
			OR email ILIKE $1
			OR first_name ILIKE $1
			OR last_name ILIKE $1
		ORDER BY created_at DESC
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
		if err := rows.Scan(&user.ID, &user.Email, &user.FirstName, &user.LastName, &user.MiddleName, &user.Position, &user.ManagerID, &user.IsActive); err != nil {
			return nil, err
		}
		user.Roles, err = s.roles(ctx, user.ID)
		if err != nil {
			return nil, err
		}
		result.Data = append(result.Data, user)
	}

	return result, rows.Err()
}

func (s *Service) Get(ctx context.Context, userID string) (*User, error) {
	var user User
	err := s.db.QueryRow(ctx, `
		SELECT id::text, email, first_name, last_name, middle_name, position, manager_id::text, is_active
		FROM users
		WHERE id = $1
	`, userID).Scan(&user.ID, &user.Email, &user.FirstName, &user.LastName, &user.MiddleName, &user.Position, &user.ManagerID, &user.IsActive)
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
		SELECT id::text, email, first_name, last_name, middle_name, position, manager_id::text, is_active
		FROM users
		WHERE manager_id = $1 AND is_active = true
		ORDER BY last_name, first_name
	`, managerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]User, 0)
	for rows.Next() {
		var user User
		if err := rows.Scan(&user.ID, &user.Email, &user.FirstName, &user.LastName, &user.MiddleName, &user.Position, &user.ManagerID, &user.IsActive); err != nil {
			return nil, err
		}
		user.Roles, err = s.roles(ctx, user.ID)
		if err != nil {
			return nil, err
		}
		result = append(result, user)
	}

	return result, rows.Err()
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
		LEFT JOIN tasks t ON t.idp_id = i.id
		WHERE i.employee_id = $1
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
		INSERT INTO users (email, password_hash, first_name, last_name, middle_name, position, manager_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id::text
	`, strings.TrimSpace(input.Email), passwordHash, input.FirstName, input.LastName, input.MiddleName, input.Position, input.ManagerID).Scan(&userID)
	if err != nil {
		if strings.Contains(err.Error(), "users_email_key") {
			return nil, ErrEmailExists
		}
		return nil, err
	}

	if err := replaceRoles(ctx, tx, userID, normalizeRoles(input.Roles)); err != nil {
		return nil, err
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
			manager_id = $6,
			is_active = $7,
			updated_at = NOW()
		WHERE id = $1
	`, userID, input.FirstName, input.LastName, input.MiddleName, input.Position, input.ManagerID, input.IsActive)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		return nil, ErrUserNotFound
	}

	if err := replaceRoles(ctx, tx, userID, normalizeRoles(input.Roles)); err != nil {
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
			position = $5,
			updated_at = NOW()
		WHERE id = $1 AND is_active = true
	`, userID, input.FirstName, input.LastName, input.MiddleName, input.Position)
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

	_, err = s.db.Exec(ctx, `
		UPDATE users
		SET password_hash = $2,
			failed_login_attempts = 0,
			locked_until = NULL,
			updated_at = NOW()
		WHERE id = $1
	`, userID, newHash)
	return err
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
