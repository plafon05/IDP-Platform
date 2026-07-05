package idp

import (
	"testing"
	"time"
)

func TestValidTransition(t *testing.T) {
	tests := []struct {
		name    string
		current string
		next    string
		want    bool
	}{
		{name: "draft to active", current: "draft", next: "active", want: true},
		{name: "draft to cancelled", current: "draft", next: "cancelled", want: true},
		{name: "active to completed", current: "active", next: "completed", want: true},
		{name: "active to cancelled", current: "active", next: "cancelled", want: true},
		{name: "active to draft", current: "active", next: "draft", want: false},
		{name: "completed to active", current: "completed", next: "active", want: false},
		{name: "same status", current: "draft", next: "draft", want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := validTransition(test.current, test.next); got != test.want {
				t.Fatalf("validTransition(%q, %q) = %v, want %v", test.current, test.next, got, test.want)
			}
		})
	}
}

func TestPlanAccess(t *testing.T) {
	plan := &Plan{EmployeeID: "employee", ManagerID: "manager"}

	if !canRead(Access{UserID: "employee"}, plan) {
		t.Fatal("employee must be able to read own IDP")
	}
	if canManage(Access{UserID: "employee"}, plan) {
		t.Fatal("employee must not be able to manage own IDP")
	}
	if !canRead(Access{UserID: "manager", Manager: true}, plan) {
		t.Fatal("responsible manager must be able to read IDP")
	}
	if !canManage(Access{UserID: "manager", Manager: true}, plan) {
		t.Fatal("responsible manager must be able to manage IDP")
	}
	if canRead(Access{UserID: "other-manager", Manager: true}, plan) {
		t.Fatal("unrelated manager must not be able to read IDP")
	}
	if !canManage(Access{UserID: "hr", IsHR: true}, plan) {
		t.Fatal("HR must be able to manage IDP")
	}
}

func TestValidateInput(t *testing.T) {
	valid := Input{
		EmployeeID: "employee",
		Title:      "Development plan",
		StartDate:  time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC),
		EndDate:    time.Date(2026, time.December, 31, 0, 0, 0, 0, time.UTC),
	}
	if err := validateInput(valid); err != nil {
		t.Fatalf("valid input rejected: %v", err)
	}

	invalidPeriod := valid
	invalidPeriod.EndDate = invalidPeriod.StartDate.AddDate(0, 0, -1)
	if err := validateInput(invalidPeriod); err != ErrInvalidInput {
		t.Fatalf("invalid period error = %v, want %v", err, ErrInvalidInput)
	}

	tooLong := valid
	goals := string(make([]rune, 10001))
	tooLong.Goals = &goals
	if err := validateInput(tooLong); err != ErrInvalidInput {
		t.Fatalf("long goals error = %v, want %v", err, ErrInvalidInput)
	}
}

func TestIDPOrderByWhitelist(t *testing.T) {
	for _, test := range []struct{ sort, order string }{
		{"created_at", "desc"}, {"start_date", "desc"}, {"end_date", "asc"}, {"progress", "desc"},
		{"employee", "asc"}, {"status", "asc"}, {"", ""},
	} {
		if value, err := idpOrderBy(test.sort, test.order); err != nil || value == "" {
			t.Fatalf("idpOrderBy(%q,%q) rejected: %v", test.sort, test.order, err)
		}
	}
	if _, err := idpOrderBy("title; DROP TABLE idps", "asc"); err != ErrInvalidInput {
		t.Fatal("unknown sort must be rejected")
	}
	if _, err := idpOrderBy("created_at", "sideways"); err != ErrInvalidInput {
		t.Fatal("unknown order must be rejected")
	}
}
