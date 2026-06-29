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

	"idp-platform/backend/internal/auth"
	"idp-platform/backend/internal/config"
	"idp-platform/backend/internal/handler"
	"idp-platform/backend/internal/migrations"
	"idp-platform/backend/internal/notification"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type fixture struct {
	db         *pgxpool.Pool
	router     http.Handler
	cfg        config.Config
	employeeID string
	managerID  string
	outsiderID string
	hrID       string
	idpID      string
	taskID     string
	publisher  *recordingPublisher
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
		AppEnv: "test", DatabaseURL: dsn, JWTSecret: "integration-test-secret",
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
	})

	t.Run("only manager edits plan", func(t *testing.T) {
		payload := map[string]any{
			"employee_id": f.employeeID, "title": "Updated plan", "goals": "**Goal**",
			"start_date": "2026-01-01", "end_date": "2026-12-31", "competencies": []any{},
		}
		f.request(t, employeeToken, http.MethodPut, "/api/v1/idps/"+f.idpID, payload, http.StatusForbidden)
		f.request(t, managerToken, http.MethodPut, "/api/v1/idps/"+f.idpID, payload, http.StatusOK)
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
		f.request(t, managerToken, http.MethodPatch, "/api/v1/idps/"+f.idpID+"/status", map[string]string{"status": "completed"}, http.StatusOK)
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
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
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
