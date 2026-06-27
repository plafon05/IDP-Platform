package tasks

import (
	"testing"
	"time"

	"idp-platform/backend/internal/idp"
)

func TestTaskAccess(t *testing.T) {
	plan := &planAccess{EmployeeID: "employee", ManagerID: "manager"}
	if !canRead(idp.Access{UserID: "employee"}, plan) {
		t.Fatal("employee must read own tasks")
	}
	if !canManage(idp.Access{UserID: "manager", Manager: true}, plan) {
		t.Fatal("direct manager must manage tasks")
	}
	if canManage(idp.Access{UserID: "employee"}, plan) {
		t.Fatal("employee must not manage task definitions")
	}
	if canRead(idp.Access{UserID: "other", Manager: true}, plan) {
		t.Fatal("unrelated manager must not read tasks")
	}
}

func TestValidateInput(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 3, 0)
	plan := &planAccess{StartDate: start, EndDate: end}
	valid := Input{Title: "Course", Priority: "medium", Status: "in_progress", Progress: 30}
	if err := validateInput(valid, plan); err != nil {
		t.Fatalf("valid input rejected: %v", err)
	}

	outside := end.AddDate(0, 0, 1)
	valid.DueDate = &outside
	if err := validateInput(valid, plan); err != ErrInvalidInput {
		t.Fatal("date outside IDP must be rejected")
	}

	valid.DueDate = nil
	valid.Status = "completed"
	if err := validateInput(valid, plan); err != ErrInvalidInput {
		t.Fatal("completed task must have 100 progress")
	}
}

func TestEditablePlan(t *testing.T) {
	if !editablePlan("draft") || !editablePlan("active") {
		t.Fatal("draft and active plans must be editable")
	}
	if editablePlan("completed") || editablePlan("cancelled") {
		t.Fatal("closed plans must not be editable")
	}
}

func TestManagerCannotChangeProgress(t *testing.T) {
	input := Input{Status: "completed", Progress: 100}
	created := normalizeNewTask(input)
	if created.Status != "not_started" || created.Progress != 0 {
		t.Fatal("new task progress must be reset")
	}

	current := &Task{Status: "in_progress", Progress: 40}
	updated, err := normalizeManagerUpdate(current, input)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Status != "in_progress" || updated.Progress != 40 {
		t.Fatal("manager update must preserve employee progress")
	}
}

func TestManagerReviewOnlyForCompletedTask(t *testing.T) {
	rating := "met"
	_, err := normalizeManagerUpdate(&Task{Status: "in_progress"}, Input{ManagerRating: &rating})
	if err != ErrInvalidInput {
		t.Fatal("review before completion must be rejected")
	}
}

func TestTaskOrderByWhitelist(t *testing.T) {
	value, err := taskOrderBy("priority", "desc")
	if err != nil || value == "" {
		t.Fatal("valid sorting rejected")
	}
	if _, err := taskOrderBy("title; DROP TABLE tasks", "asc"); err != ErrInvalidInput {
		t.Fatal("unknown sort must be rejected")
	}
	if _, err := taskOrderBy("due_date", "sideways"); err != ErrInvalidInput {
		t.Fatal("unknown order must be rejected")
	}
}
