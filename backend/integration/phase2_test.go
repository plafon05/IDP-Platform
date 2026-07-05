package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"idp-platform/backend/internal/analytics"
	"idp-platform/backend/internal/auth"
	"idp-platform/backend/internal/config"
	"idp-platform/backend/internal/handler"
	"idp-platform/backend/internal/idp"
	"idp-platform/backend/internal/migrations"
	"idp-platform/backend/internal/notification"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type fixture struct {
	db          *pgxpool.Pool
	router      http.Handler
	cfg         config.Config
	employeeID  string
	managerID   string
	outsiderID  string
	hrID        string
	idpID       string
	taskID      string
	otherIDPID  string
	otherTaskID string
	publisher   *recordingPublisher
}

type recordingPublisher struct{ jobs []notification.Job }

func (p *recordingPublisher) Enqueue(_ context.Context, job notification.Job) error {
	p.jobs = append(p.jobs, job)
	return nil
}

func (p *recordingPublisher) EnqueueTx(_ context.Context, _ pgx.Tx, job notification.Job) error {
	p.jobs = append(p.jobs, job)
	return nil
}

func TestPhase2RoleMatrix(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	dbConfig, err := pgx.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(dbConfig.Database, "_test") {
		t.Fatalf("integration tests require a database ending in _test, got %q", dbConfig.Database)
	}

	cfg := config.Config{
		AppEnv: "test", DatabaseURL: dsn,
		JWTSecrets:   []config.JWTSigningKey{{KeyID: "integration", Secret: "integration-test-secret"}},
		JWTAccessTTL: time.Hour, JWTRefreshTTL: time.Hour, CORSOrigins: []string{"http://example.test"},
	}
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	current, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	if err := migrations.Up(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(current); err != nil {
		t.Fatal(err)
	}

	db, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	f := &fixture{db: db, cfg: cfg, publisher: &recordingPublisher{}}
	f.reset(t)
	defer f.reset(t)
	f.seed(t)
	f.router = handler.NewRouter(cfg, db, nil, f.publisher)

	employeeToken := f.token(t, f.employeeID, "employee")
	managerToken := f.token(t, f.managerID, "employee", "manager")
	outsiderToken := f.token(t, f.outsiderID, "employee", "manager")
	hrToken := f.token(t, f.hrID, "employee", "hr_admin")

	t.Run("object access", func(t *testing.T) {
		f.request(t, employeeToken, http.MethodGet, "/api/v1/idps/"+f.idpID, nil, http.StatusOK)
		f.request(t, managerToken, http.MethodGet, "/api/v1/idps/"+f.idpID, nil, http.StatusOK)
		f.request(t, hrToken, http.MethodGet, "/api/v1/idps/"+f.idpID, nil, http.StatusOK)
		f.request(t, outsiderToken, http.MethodGet, "/api/v1/idps/"+f.idpID, nil, http.StatusForbidden)
		profilePath := "/api/v1/employees/" + f.employeeID + "/profile"
		f.request(t, employeeToken, http.MethodGet, profilePath, nil, http.StatusOK)
		f.request(t, managerToken, http.MethodGet, profilePath, nil, http.StatusOK)
		f.request(t, hrToken, http.MethodGet, profilePath, nil, http.StatusOK)
		f.request(t, outsiderToken, http.MethodGet, profilePath, nil, http.StatusForbidden)
	})

	t.Run("department hierarchy", func(t *testing.T) {
		f.request(t, employeeToken, http.MethodGet, "/api/v1/departments", nil, http.StatusOK)
		f.request(t, employeeToken, http.MethodPost, "/api/v1/departments", map[string]any{"name": "Root"}, http.StatusForbidden)
		response := f.request(t, hrToken, http.MethodPost, "/api/v1/departments", map[string]any{"name": "Root"}, http.StatusCreated)
		var parent struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &parent); err != nil {
			t.Fatal(err)
		}
		response = f.request(t, hrToken, http.MethodPost, "/api/v1/departments", map[string]any{"name": "Child", "parent_id": parent.ID}, http.StatusCreated)
		var child struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &child); err != nil {
			t.Fatal(err)
		}
		f.request(t, hrToken, http.MethodDelete, "/api/v1/departments/"+parent.ID, nil, http.StatusConflict)
		f.request(t, hrToken, http.MethodDelete, "/api/v1/departments/"+child.ID, nil, http.StatusNoContent)
		f.request(t, hrToken, http.MethodDelete, "/api/v1/departments/"+parent.ID, nil, http.StatusNoContent)
	})

	t.Run("analytics scope", func(t *testing.T) {
		path := "/api/v1/analytics/overview?from=2025-01-01&to=2026-12-31"
		f.request(t, employeeToken, http.MethodGet, path, nil, http.StatusForbidden)
		managerResponse := f.request(t, managerToken, http.MethodGet, path, nil, http.StatusOK)
		hrResponse := f.request(t, hrToken, http.MethodGet, path, nil, http.StatusOK)
		outsiderResponse := f.request(t, outsiderToken, http.MethodGet, path, nil, http.StatusOK)
		var managerResult, hrResult, outsiderResult struct {
			Summary struct {
				Plans int `json:"plans"`
			} `json:"summary"`
		}
		for response, target := range map[*httptest.ResponseRecorder]*struct {
			Summary struct {
				Plans int `json:"plans"`
			} `json:"summary"`
		}{managerResponse: &managerResult, hrResponse: &hrResult, outsiderResponse: &outsiderResult} {
			if err := json.Unmarshal(response.Body.Bytes(), target); err != nil {
				t.Fatal(err)
			}
		}
		if managerResult.Summary.Plans != 1 || hrResult.Summary.Plans != 2 || outsiderResult.Summary.Plans != 1 {
			t.Fatalf("unexpected analytics scope: manager=%d hr=%d outsider=%d", managerResult.Summary.Plans, hrResult.Summary.Plans, outsiderResult.Summary.Plans)
		}
	})

	t.Run("analytics service aggregates", func(t *testing.T) {
		service := analytics.NewService(f.db)
		period := analytics.Filters{From: date(t, "2025-01-01"), To: date(t, "2026-12-31")}

		t.Run("manager sees only own employees", func(t *testing.T) {
			result, err := service.Overview(context.Background(), idp.Access{UserID: f.managerID, Manager: true}, period)
			if err != nil {
				t.Fatal(err)
			}
			if result.Summary != (analytics.Summary{Plans: 1, Employees: 1, Tasks: 1, AverageProgress: 20}) || len(result.Employees) != 1 {
				t.Fatalf("unexpected manager analytics: %+v employees=%d", result.Summary, len(result.Employees))
			}
		})

		t.Run("hr sees organization aggregates", func(t *testing.T) {
			result, err := service.Overview(context.Background(), idp.Access{UserID: f.hrID, IsHR: true}, period)
			if err != nil {
				t.Fatal(err)
			}
			if result.Summary != (analytics.Summary{Plans: 2, Employees: 2, Tasks: 2, AverageProgress: 50}) || len(result.Statuses) != 2 || len(result.Employees) != 2 {
				t.Fatalf("unexpected HR analytics: %+v statuses=%#v employees=%d", result.Summary, result.Statuses, len(result.Employees))
			}
		})

		t.Run("date and status filters are applied", func(t *testing.T) {
			result, err := service.Overview(context.Background(), idp.Access{UserID: f.hrID, IsHR: true}, analytics.Filters{
				From: date(t, "2026-01-01"), To: date(t, "2026-12-31"), Status: "active",
			})
			if err != nil {
				t.Fatal(err)
			}
			if result.Summary != (analytics.Summary{Plans: 1, Employees: 1, Tasks: 1, AverageProgress: 20}) || len(result.Statuses) != 1 || result.Statuses[0].Name != "active" {
				t.Fatalf("unexpected filtered analytics: %+v statuses=%#v", result.Summary, result.Statuses)
			}
		})
	})

	t.Run("template creates draft plan", func(t *testing.T) {
		f.request(t, employeeToken, http.MethodGet, "/api/v1/idp-templates", nil, http.StatusForbidden)
		response := f.request(t, managerToken, http.MethodPost, "/api/v1/idp-templates", map[string]any{
			"title": "Backend template", "goals": "Grow skills", "is_active": true,
			"tasks":        []map[string]any{{"title": "Course", "priority": "medium", "due_offset_days": 30}},
			"competencies": []any{},
		}, http.StatusCreated)
		var template struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &template); err != nil {
			t.Fatal(err)
		}
		response = f.request(t, managerToken, http.MethodPost, "/api/v1/idp-templates/"+template.ID+"/apply", map[string]any{
			"employee_id": f.employeeID, "title": "Plan from template", "start_date": "2026-02-01", "end_date": "2026-06-01",
		}, http.StatusCreated)
		var plan struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &plan); err != nil {
			t.Fatal(err)
		}
		var status string
		var tasks int
		if err := f.db.QueryRow(context.Background(), `SELECT i.status,COUNT(t.id)::int FROM idps i LEFT JOIN tasks t ON t.idp_id=i.id WHERE i.id=$1 GROUP BY i.id`, plan.ID).Scan(&status, &tasks); err != nil {
			t.Fatal(err)
		}
		if status != "draft" || tasks != 1 {
			t.Fatalf("unexpected template result: status=%s tasks=%d", status, tasks)
		}
	})

	t.Run("only manager edits plan", func(t *testing.T) {
		payload := map[string]any{
			"employee_id": f.employeeID, "title": "Updated plan", "goals": "**Goal**",
			"start_date": "2026-01-01", "end_date": "2026-12-31", "competencies": []any{},
		}
		f.request(t, employeeToken, http.MethodPut, "/api/v1/idps/"+f.idpID, payload, http.StatusForbidden)
		f.request(t, managerToken, http.MethodPut, "/api/v1/idps/"+f.idpID, payload, http.StatusOK)
	})

	t.Run("task uses category and tag", func(t *testing.T) {
		var categoryID, tagID string
		if err := f.db.QueryRow(context.Background(), `INSERT INTO task_categories(name) VALUES('Course') ON CONFLICT(name) DO UPDATE SET name=EXCLUDED.name RETURNING id::text`).Scan(&categoryID); err != nil {
			t.Fatal(err)
		}
		if err := f.db.QueryRow(context.Background(), `INSERT INTO tags(name) VALUES('Backend') ON CONFLICT(name) DO UPDATE SET name=EXCLUDED.name RETURNING id::text`).Scan(&tagID); err != nil {
			t.Fatal(err)
		}
		response := f.request(t, managerToken, http.MethodPost, "/api/v1/idps/"+f.idpID+"/tasks", map[string]any{
			"title": "Task with references", "category_id": categoryID, "priority": "medium",
			"status": "not_started", "progress": 0, "competency_ids": []any{}, "tag_ids": []string{tagID}, "resources": []any{},
		}, http.StatusCreated)
		var task struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &task); err != nil {
			t.Fatal(err)
		}
		f.request(t, managerToken, http.MethodDelete, "/api/v1/tasks/"+task.ID, nil, http.StatusNoContent)
	})

	t.Run("only employee reports progress", func(t *testing.T) {
		payload := map[string]any{"status": "in_progress", "progress": 50}
		f.request(t, managerToken, http.MethodPatch, "/api/v1/tasks/"+f.taskID+"/progress", payload, http.StatusForbidden)
		f.request(t, employeeToken, http.MethodPatch, "/api/v1/tasks/"+f.taskID+"/progress", payload, http.StatusOK)
	})

	t.Run("manager cannot overwrite progress", func(t *testing.T) {
		payload := map[string]any{
			"title": "Updated task", "priority": "high", "status": "completed", "progress": 100,
			"competency_ids": []any{}, "tag_ids": []any{}, "resources": []any{},
		}
		f.request(t, managerToken, http.MethodPut, "/api/v1/tasks/"+f.taskID, payload, http.StatusOK)
		var status string
		var progress int
		if err := f.db.QueryRow(context.Background(), `SELECT status,progress FROM tasks WHERE id=$1`, f.taskID).Scan(&status, &progress); err != nil {
			t.Fatal(err)
		}
		if status != "in_progress" || progress != 50 {
			t.Fatalf("manager changed employee progress: status=%s progress=%d", status, progress)
		}
	})

	t.Run("comment ownership", func(t *testing.T) {
		f.publisher.jobs = nil
		response := f.request(t, employeeToken, http.MethodPost, "/api/v1/tasks/"+f.taskID+"/comments", map[string]string{"content": "Employee comment"}, http.StatusCreated)
		if len(f.publisher.jobs) != 1 || f.publisher.jobs[0].Template != notification.CommentCreatedTemplate || f.publisher.jobs[0].To[0] != "manager@test.local" {
			t.Fatalf("unexpected comment notifications: %#v", f.publisher.jobs)
		}
		var comment struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &comment); err != nil {
			t.Fatal(err)
		}
		f.request(t, outsiderToken, http.MethodGet, "/api/v1/tasks/"+f.taskID+"/comments", nil, http.StatusForbidden)
		f.request(t, managerToken, http.MethodPut, "/api/v1/comments/"+comment.ID, map[string]string{"content": "Changed"}, http.StatusForbidden)
		f.request(t, employeeToken, http.MethodPut, "/api/v1/comments/"+comment.ID, map[string]string{"content": "Changed by author"}, http.StatusOK)
		f.request(t, employeeToken, http.MethodDelete, "/api/v1/comments/"+comment.ID, nil, http.StatusNoContent)
	})

	t.Run("audit and optional completion comment", func(t *testing.T) {
		f.request(t, employeeToken, http.MethodGet, "/api/v1/tasks/"+f.taskID+"/audit", nil, http.StatusOK)
		f.request(t, managerToken, http.MethodPatch, "/api/v1/idps/"+f.idpID+"/status", map[string]string{"status": "completed"}, http.StatusConflict)
		f.request(t, employeeToken, http.MethodPatch, "/api/v1/tasks/"+f.taskID+"/progress", map[string]any{"status": "completed", "progress": 100}, http.StatusOK)
		f.request(t, managerToken, http.MethodPatch, "/api/v1/idps/"+f.idpID+"/status", map[string]string{"status": "completed"}, http.StatusOK)
	})

	t.Run("idor protection", func(t *testing.T) {
		foreignTaskPath := "/api/v1/tasks/" + f.otherTaskID
		foreignIDPPath := "/api/v1/idps/" + f.otherIDPID
		taskPayload := map[string]any{
			"title": "Updated task", "priority": "medium", "status": "in_progress", "progress": 30,
			"competency_ids": []any{}, "tag_ids": []any{}, "resources": []any{},
		}
		idpPayload := map[string]any{
			"employee_id": f.outsiderID, "title": "Unauthorized update",
			"start_date": "2027-01-01", "end_date": "2027-12-31", "competencies": []any{},
		}

		t.Run("employee cannot access another employee task", func(t *testing.T) {
			f.request(t, employeeToken, http.MethodGet, foreignTaskPath, nil, http.StatusForbidden)
			f.request(t, employeeToken, http.MethodGet, foreignIDPPath+"/tasks", nil, http.StatusForbidden)
			f.request(t, employeeToken, http.MethodPatch, foreignTaskPath+"/progress", map[string]any{"status": "completed", "progress": 100}, http.StatusForbidden)
			f.request(t, employeeToken, http.MethodGet, foreignTaskPath+"/audit", nil, http.StatusForbidden)
		})

		t.Run("unrelated manager cannot read or mutate task", func(t *testing.T) {
			f.request(t, managerToken, http.MethodGet, foreignTaskPath, nil, http.StatusForbidden)
			f.request(t, managerToken, http.MethodPut, foreignTaskPath, taskPayload, http.StatusForbidden)
			f.request(t, managerToken, http.MethodDelete, foreignTaskPath, nil, http.StatusForbidden)
			f.request(t, managerToken, http.MethodGet, foreignTaskPath+"/audit", nil, http.StatusForbidden)
		})

		for name, token := range map[string]string{"employee": employeeToken, "unrelated manager": managerToken} {
			t.Run(name+" cannot access foreign IDP", func(t *testing.T) {
				f.request(t, token, http.MethodGet, foreignIDPPath, nil, http.StatusForbidden)
				f.request(t, token, http.MethodPut, foreignIDPPath, idpPayload, http.StatusForbidden)
				f.request(t, token, http.MethodPatch, foreignIDPPath+"/status", map[string]string{"status": "cancelled", "reason": "unauthorized"}, http.StatusForbidden)
				f.request(t, token, http.MethodDelete, foreignIDPPath, nil, http.StatusForbidden)
				f.request(t, token, http.MethodGet, foreignIDPPath+"/audit", nil, http.StatusForbidden)
			})
		}

		for name, token := range map[string]string{"employee": employeeToken, "unrelated manager": managerToken} {
			t.Run(name+" cannot access foreign comments", func(t *testing.T) {
				f.request(t, token, http.MethodGet, foreignIDPPath+"/comments", nil, http.StatusForbidden)
				f.request(t, token, http.MethodPost, foreignIDPPath+"/comments", map[string]string{"content": "unauthorized"}, http.StatusForbidden)
				f.request(t, token, http.MethodGet, foreignTaskPath+"/comments", nil, http.StatusForbidden)
				f.request(t, token, http.MethodPost, foreignTaskPath+"/comments", map[string]string{"content": "unauthorized"}, http.StatusForbidden)
			})
		}

		t.Run("hr has organization wide access", func(t *testing.T) {
			f.request(t, hrToken, http.MethodGet, foreignTaskPath, nil, http.StatusOK)
			f.request(t, hrToken, http.MethodPut, foreignTaskPath, taskPayload, http.StatusOK)
			f.request(t, hrToken, http.MethodGet, foreignTaskPath+"/audit", nil, http.StatusOK)
			f.request(t, hrToken, http.MethodGet, foreignIDPPath, nil, http.StatusOK)
			f.request(t, hrToken, http.MethodPut, foreignIDPPath, idpPayload, http.StatusOK)
			f.request(t, hrToken, http.MethodGet, foreignIDPPath+"/audit", nil, http.StatusOK)
			f.request(t, hrToken, http.MethodGet, foreignIDPPath+"/comments", nil, http.StatusOK)
			f.request(t, hrToken, http.MethodGet, foreignTaskPath+"/comments", nil, http.StatusOK)
			f.request(t, hrToken, http.MethodDelete, foreignIDPPath, nil, http.StatusNoContent)
		})
	})
}

func (f *fixture) seed(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	hash, err := auth.HashPassword("TestPassword1")
	if err != nil {
		t.Fatal(err)
	}
	tx, err := f.db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)

	f.managerID = insertUser(t, tx, "manager@test.local", hash, "Manager", "One", "Lead")
	f.employeeID = insertUser(t, tx, "employee@test.local", hash, "Employee", "One", "Developer")
	f.outsiderID = insertUser(t, tx, "outsider@test.local", hash, "Manager", "Other", "Lead")
	f.hrID = insertUser(t, tx, "hr@test.local", hash, "HR", "Admin", "HR")
	if _, err := tx.Exec(ctx, `UPDATE users SET manager_id=$1 WHERE id=$2`, f.managerID, f.employeeID); err != nil {
		t.Fatal(err)
	}
	insertRoles(t, tx, f.managerID, "employee", "manager")
	insertRoles(t, tx, f.employeeID, "employee")
	insertRoles(t, tx, f.outsiderID, "employee", "manager")
	insertRoles(t, tx, f.hrID, "employee", "hr_admin")
	if err := tx.QueryRow(ctx, `INSERT INTO idps(employee_id,manager_id,title,start_date,end_date,status) VALUES($1,$2,'Test plan','2026-01-01','2026-12-31','active') RETURNING id::text`, f.employeeID, f.managerID).Scan(&f.idpID); err != nil {
		t.Fatal(err)
	}
	if err := tx.QueryRow(ctx, `INSERT INTO tasks(idp_id,title,priority,status,progress) VALUES($1,'Test task','medium','in_progress',20) RETURNING id::text`, f.idpID).Scan(&f.taskID); err != nil {
		t.Fatal(err)
	}
	var analyticsPlanID string
	if err := tx.QueryRow(ctx, `INSERT INTO idps(employee_id,manager_id,title,start_date,end_date,status) VALUES($1,$1,'Analytics plan','2025-01-01','2025-12-31','completed') RETURNING id::text`, f.outsiderID).Scan(&analyticsPlanID); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO tasks(idp_id,title,priority,status,progress) VALUES($1,'Analytics task','medium','completed',80)`, analyticsPlanID); err != nil {
		t.Fatal(err)
	}
	if err := tx.QueryRow(ctx, `INSERT INTO idps(employee_id,manager_id,title,start_date,end_date,status) VALUES($1,$1,'Foreign plan','2027-01-01','2027-12-31','active') RETURNING id::text`, f.outsiderID).Scan(&f.otherIDPID); err != nil {
		t.Fatal(err)
	}
	if err := tx.QueryRow(ctx, `INSERT INTO tasks(idp_id,title,priority,status,progress) VALUES($1,'Foreign task','medium','in_progress',30) RETURNING id::text`, f.otherIDPID).Scan(&f.otherTaskID); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
}

func date(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := time.Parse(time.DateOnly, value)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}

func (f *fixture) reset(t *testing.T) {
	t.Helper()
	if _, err := f.db.Exec(context.Background(), `TRUNCATE users CASCADE`); err != nil {
		t.Fatal(err)
	}
}

func insertUser(t *testing.T, tx pgx.Tx, email, hash, firstName, lastName, position string) string {
	t.Helper()
	var id string
	if err := tx.QueryRow(context.Background(), `INSERT INTO users(email,password_hash,first_name,last_name,position) VALUES($1,$2,$3,$4,$5) RETURNING id::text`, email, hash, firstName, lastName, position).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func insertRoles(t *testing.T, tx pgx.Tx, userID string, roles ...string) {
	t.Helper()
	for _, role := range roles {
		if _, err := tx.Exec(context.Background(), `INSERT INTO user_roles(user_id,role) VALUES($1,$2)`, userID, role); err != nil {
			t.Fatal(err)
		}
	}
}

func (f *fixture) token(t *testing.T, userID string, roles ...string) string {
	t.Helper()
	token, _, err := auth.GenerateAccessToken(f.cfg, userID, roles)
	if err != nil {
		t.Fatal(err)
	}
	return token
}

func (f *fixture) request(t *testing.T, token, method, path string, payload any, wantStatus int) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	if payload != nil {
		if err := json.NewEncoder(&body).Encode(payload); err != nil {
			t.Fatal(err)
		}
	}
	req := httptest.NewRequest(method, path, &body)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	f.router.ServeHTTP(response, req)
	if response.Code != wantStatus {
		t.Fatalf("%s %s: status=%d want=%d body=%s", method, path, response.Code, wantStatus, response.Body.String())
	}
	return response
}
