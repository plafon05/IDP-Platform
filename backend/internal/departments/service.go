package departments

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound = errors.New("department not found")
	ErrInvalid  = errors.New("invalid department hierarchy")
	ErrInUse    = errors.New("department is in use")
)

type Service struct{ db *pgxpool.Pool }
type Employee struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Position string `json:"position"`
}
type Department struct {
	ID        string        `json:"id"`
	Name      string        `json:"name"`
	ParentID  *string       `json:"parent_id,omitempty"`
	Depth     int           `json:"depth"`
	Employees []Employee    `json:"employees"`
	Children  []*Department `json:"children"`
}
type Input struct {
	Name     string
	ParentID *string
}

func NewService(db *pgxpool.Pool) *Service { return &Service{db: db} }

func (s *Service) Tree(ctx context.Context) ([]*Department, error) {
	rows, err := s.db.Query(ctx, `SELECT id::text,name,parent_id::text,nlevel(path) FROM departments ORDER BY path,name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	all := map[string]*Department{}
	order := []*Department{}
	for rows.Next() {
		item := &Department{Employees: []Employee{}, Children: []*Department{}}
		if err := rows.Scan(&item.ID, &item.Name, &item.ParentID, &item.Depth); err != nil {
			return nil, err
		}
		all[item.ID] = item
		order = append(order, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	employees, err := s.db.Query(ctx, `SELECT id::text,department_id::text,concat_ws(' ',last_name,first_name,middle_name),position FROM users WHERE department_id IS NOT NULL ORDER BY last_name,first_name`)
	if err != nil {
		return nil, err
	}
	defer employees.Close()
	for employees.Next() {
		var item Employee
		var departmentID string
		if err := employees.Scan(&item.ID, &departmentID, &item.Name, &item.Position); err != nil {
			return nil, err
		}
		if department := all[departmentID]; department != nil {
			department.Employees = append(department.Employees, item)
		}
	}
	if err := employees.Err(); err != nil {
		return nil, err
	}
	roots := []*Department{}
	for _, item := range order {
		if item.ParentID == nil {
			roots = append(roots, item)
		} else if parent := all[*item.ParentID]; parent != nil {
			parent.Children = append(parent.Children, item)
		}
	}
	return roots, nil
}

func (s *Service) Create(ctx context.Context, input Input) (*Department, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, ErrInvalid
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	parentPath := ""
	if input.ParentID != nil {
		if err := tx.QueryRow(ctx, `SELECT path::text FROM departments WHERE id=$1 AND nlevel(path)<5`, *input.ParentID).Scan(&parentPath); errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrInvalid
		} else if err != nil {
			return nil, err
		}
	}
	var id string
	if err := tx.QueryRow(ctx, `INSERT INTO departments(name,parent_id) VALUES($1,$2) RETURNING id::text`, name, input.ParentID).Scan(&id); err != nil {
		return nil, err
	}
	label := strings.ReplaceAll(id, "-", "")
	path := label
	if parentPath != "" {
		path = parentPath + "." + label
	}
	if _, err := tx.Exec(ctx, `UPDATE departments SET path=$2::ltree WHERE id=$1`, id, path); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &Department{ID: id, Name: name, ParentID: input.ParentID, Depth: strings.Count(path, ".") + 1, Employees: []Employee{}, Children: []*Department{}}, nil
}

func (s *Service) Update(ctx context.Context, id string, input Input) error {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return ErrInvalid
	}
	if input.ParentID != nil && *input.ParentID == id {
		return ErrInvalid
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var oldPath string
	if err := tx.QueryRow(ctx, `SELECT path::text FROM departments WHERE id=$1 FOR UPDATE`, id).Scan(&oldPath); errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	} else if err != nil {
		return err
	}
	parentPath := ""
	parentDepth := 0
	if input.ParentID != nil {
		if err := tx.QueryRow(ctx, `SELECT path::text,nlevel(path) FROM departments WHERE id=$1 AND NOT(path <@ $2::ltree)`, *input.ParentID, oldPath).Scan(&parentPath, &parentDepth); errors.Is(err, pgx.ErrNoRows) {
			return ErrInvalid
		} else if err != nil {
			return err
		}
	}
	var subtreeDepth int
	if err := tx.QueryRow(ctx, `SELECT COALESCE(MAX(nlevel(path)-nlevel($1::ltree)),0) FROM departments WHERE path <@ $1::ltree`, oldPath).Scan(&subtreeDepth); err != nil {
		return err
	}
	if parentDepth+1+subtreeDepth > 5 {
		return ErrInvalid
	}
	label := strings.ReplaceAll(id, "-", "")
	newPath := label
	if parentPath != "" {
		newPath = parentPath + "." + label
	}
	if _, err := tx.Exec(ctx, `UPDATE departments SET path=CASE WHEN path=$2::ltree THEN $1::ltree ELSE $1::ltree || subpath(path,nlevel($2::ltree)) END WHERE path <@ $2::ltree`, newPath, oldPath); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE departments SET name=$2,parent_id=$3 WHERE id=$1`, id, name, input.ParentID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Service) Delete(ctx context.Context, id string) error {
	var blocked bool
	err := s.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM departments WHERE parent_id=$1) OR EXISTS(SELECT 1 FROM users WHERE department_id=$1)`, id).Scan(&blocked)
	if err != nil {
		return err
	}
	if blocked {
		return ErrInUse
	}
	tag, err := s.db.Exec(ctx, `DELETE FROM departments WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
