package comments

import (
	"testing"
	"time"

	"idp-platform/backend/internal/idp"
)

func TestCommentAccess(t *testing.T) {
	entity := &entityAccess{EmployeeID: "employee", ManagerID: "manager"}
	if !canRead(idp.Access{UserID: "employee"}, entity) {
		t.Fatal("employee must read comments")
	}
	if !canRead(idp.Access{UserID: "manager", Manager: true}, entity) {
		t.Fatal("direct manager must read comments")
	}
	if canRead(idp.Access{UserID: "other", Manager: true}, entity) {
		t.Fatal("unrelated manager must not read comments")
	}
}

func TestCommentCapabilities(t *testing.T) {
	now := time.Now()
	recent := Comment{AuthorID: "author", Content: "text", CreatedAt: now.Add(-9 * time.Minute)}
	row := fakeRow{comment: recent}
	item, err := scanComment(row, "author")
	if err != nil {
		t.Fatal(err)
	}
	if !item.CanEdit || !item.CanDelete {
		t.Fatal("recent own comment must be editable and deletable")
	}

	old := recent
	old.CreatedAt = now.Add(-11 * time.Minute)
	item, err = scanComment(fakeRow{comment: old}, "author")
	if err != nil {
		t.Fatal(err)
	}
	if item.CanEdit || !item.CanDelete {
		t.Fatal("old own comment must only be deletable")
	}
}

type fakeRow struct{ comment Comment }

func (r fakeRow) Scan(dest ...any) error {
	*(dest[0].(*string)) = r.comment.ID
	*(dest[1].(*string)) = r.comment.EntityType
	*(dest[2].(*string)) = r.comment.EntityID
	*(dest[3].(*string)) = r.comment.AuthorID
	*(dest[4].(*string)) = r.comment.AuthorName
	*(dest[5].(**string)) = r.comment.AuthorAvatar
	*(dest[6].(*string)) = r.comment.Content
	*(dest[7].(*bool)) = r.comment.IsDeleted
	*(dest[8].(*time.Time)) = r.comment.CreatedAt
	*(dest[9].(*time.Time)) = r.comment.UpdatedAt
	return nil
}
