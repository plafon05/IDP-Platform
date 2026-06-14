package catalog

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound     = errors.New("catalog item not found")
	ErrNameExists   = errors.New("catalog item already exists")
	ErrInUse        = errors.New("catalog item is in use")
	ErrInvalidInput = errors.New("invalid catalog input")
)

type Service struct {
	db *pgxpool.Pool
}

type Competency struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description *string           `json:"description,omitempty"`
	Category    string            `json:"category"`
	IsActive    bool              `json:"is_active"`
	Levels      []CompetencyLevel `json:"levels,omitempty"`
}

type CompetencyLevel struct {
	Level       int     `json:"level"`
	Title       string  `json:"title"`
	Description *string `json:"description,omitempty"`
}

type CompetencyInput struct {
	Name        string
	Description *string
	Category    string
	IsActive    bool
	Levels      []CompetencyLevel
}

type NamedItem struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func NewService(db *pgxpool.Pool) *Service {
	return &Service{db: db}
}

func (s *Service) ListCompetencies(ctx context.Context, includeInactive bool) ([]Competency, error) {
	rows, err := s.db.Query(ctx, `
		SELECT id::text, name, description, category, is_active
		FROM competencies
		WHERE $1 OR is_active = true
		ORDER BY name
	`, includeInactive)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]Competency, 0)
	for rows.Next() {
		var item Competency
		if err := rows.Scan(&item.ID, &item.Name, &item.Description, &item.Category, &item.IsActive); err != nil {
			return nil, err
		}
		item.Levels, err = s.levels(ctx, item.ID)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}

	return result, rows.Err()
}

func (s *Service) CreateCompetency(ctx context.Context, input CompetencyInput) (*Competency, error) {
	if !validCategory(input.Category) {
		return nil, ErrInvalidInput
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var id string
	err = tx.QueryRow(ctx, `
		INSERT INTO competencies (name, description, category, is_active)
		VALUES ($1, $2, $3, $4)
		RETURNING id::text
	`, strings.TrimSpace(input.Name), input.Description, input.Category, input.IsActive).Scan(&id)
	if err != nil {
		if isPgCode(err, "23505") {
			return nil, ErrNameExists
		}
		return nil, err
	}

	if err := replaceLevels(ctx, tx, id, input.Levels); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return s.GetCompetency(ctx, id)
}

func (s *Service) GetCompetency(ctx context.Context, id string) (*Competency, error) {
	var item Competency
	err := s.db.QueryRow(ctx, `
		SELECT id::text, name, description, category, is_active
		FROM competencies
		WHERE id = $1
	`, id).Scan(&item.ID, &item.Name, &item.Description, &item.Category, &item.IsActive)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	item.Levels, err = s.levels(ctx, id)
	if err != nil {
		return nil, err
	}

	return &item, nil
}

func (s *Service) UpdateCompetency(ctx context.Context, id string, input CompetencyInput) (*Competency, error) {
	if !validCategory(input.Category) {
		return nil, ErrInvalidInput
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	tag, err := tx.Exec(ctx, `
		UPDATE competencies
		SET name = $2,
			description = $3,
			category = $4,
			is_active = $5
		WHERE id = $1
	`, id, strings.TrimSpace(input.Name), input.Description, input.Category, input.IsActive)
	if err != nil {
		if isPgCode(err, "23505") {
			return nil, ErrNameExists
		}
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		return nil, ErrNotFound
	}

	if err := replaceLevels(ctx, tx, id, input.Levels); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return s.GetCompetency(ctx, id)
}

func (s *Service) ArchiveCompetency(ctx context.Context, id string) error {
	tag, err := s.db.Exec(ctx, `UPDATE competencies SET is_active = false WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Service) ListTaskCategories(ctx context.Context) ([]NamedItem, error) {
	return s.listNamed(ctx, "task_categories")
}

func (s *Service) CreateTaskCategory(ctx context.Context, name string) (*NamedItem, error) {
	return s.createNamed(ctx, "task_categories", name)
}

func (s *Service) UpdateTaskCategory(ctx context.Context, id, name string) (*NamedItem, error) {
	return s.updateNamed(ctx, "task_categories", id, name)
}

func (s *Service) DeleteTaskCategory(ctx context.Context, id string) error {
	return s.deleteNamed(ctx, "task_categories", id)
}

func (s *Service) ListTags(ctx context.Context) ([]NamedItem, error) {
	return s.listNamed(ctx, "tags")
}

func (s *Service) CreateTag(ctx context.Context, name string) (*NamedItem, error) {
	return s.createNamed(ctx, "tags", name)
}

func (s *Service) UpdateTag(ctx context.Context, id, name string) (*NamedItem, error) {
	return s.updateNamed(ctx, "tags", id, name)
}

func (s *Service) DeleteTag(ctx context.Context, id string) error {
	return s.deleteNamed(ctx, "tags", id)
}

func (s *Service) levels(ctx context.Context, competencyID string) ([]CompetencyLevel, error) {
	rows, err := s.db.Query(ctx, `
		SELECT level, title, description
		FROM competency_levels
		WHERE competency_id = $1
		ORDER BY level
	`, competencyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	levels := make([]CompetencyLevel, 0, 4)
	for rows.Next() {
		var level CompetencyLevel
		if err := rows.Scan(&level.Level, &level.Title, &level.Description); err != nil {
			return nil, err
		}
		levels = append(levels, level)
	}

	return levels, rows.Err()
}

func replaceLevels(ctx context.Context, tx pgx.Tx, competencyID string, levels []CompetencyLevel) error {
	if _, err := tx.Exec(ctx, `DELETE FROM competency_levels WHERE competency_id = $1`, competencyID); err != nil {
		return err
	}

	seen := map[int]bool{}
	for _, level := range levels {
		if level.Level < 1 || level.Level > 4 || strings.TrimSpace(level.Title) == "" || seen[level.Level] {
			return ErrInvalidInput
		}
		seen[level.Level] = true

		if _, err := tx.Exec(ctx, `
			INSERT INTO competency_levels (competency_id, level, title, description)
			VALUES ($1, $2, $3, $4)
		`, competencyID, level.Level, strings.TrimSpace(level.Title), level.Description); err != nil {
			return err
		}
	}

	return nil
}

func (s *Service) listNamed(ctx context.Context, table string) ([]NamedItem, error) {
	rows, err := s.db.Query(ctx, fmt.Sprintf(`
		SELECT id::text, name
		FROM %s
		ORDER BY name
	`, table))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]NamedItem, 0)
	for rows.Next() {
		var item NamedItem
		if err := rows.Scan(&item.ID, &item.Name); err != nil {
			return nil, err
		}
		result = append(result, item)
	}

	return result, rows.Err()
}

func (s *Service) createNamed(ctx context.Context, table, name string) (*NamedItem, error) {
	var item NamedItem
	err := s.db.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s (name)
		VALUES ($1)
		RETURNING id::text, name
	`, table), strings.TrimSpace(name)).Scan(&item.ID, &item.Name)
	if err != nil {
		if isPgCode(err, "23505") {
			return nil, ErrNameExists
		}
		return nil, err
	}

	return &item, nil
}

func (s *Service) updateNamed(ctx context.Context, table, id, name string) (*NamedItem, error) {
	var item NamedItem
	err := s.db.QueryRow(ctx, fmt.Sprintf(`
		UPDATE %s
		SET name = $2
		WHERE id = $1
		RETURNING id::text, name
	`, table), id, strings.TrimSpace(name)).Scan(&item.ID, &item.Name)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		if isPgCode(err, "23505") {
			return nil, ErrNameExists
		}
		return nil, err
	}

	return &item, nil
}

func (s *Service) deleteNamed(ctx context.Context, table, id string) error {
	tag, err := s.db.Exec(ctx, fmt.Sprintf(`DELETE FROM %s WHERE id = $1`, table), id)
	if err != nil {
		if isPgCode(err, "23503") {
			return ErrInUse
		}
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func isPgCode(err error, code string) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == code
}

func validCategory(category string) bool {
	switch category {
	case "hard", "soft", "leadership", "management", "technical":
		return true
	default:
		return false
	}
}
