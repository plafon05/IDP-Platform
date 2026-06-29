package notification

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type ReminderScheduler struct {
	db          *pgxpool.Pool
	publisher   Publisher
	frontendURL string
	location    *time.Location
}

func NewReminderScheduler(db *pgxpool.Pool, publisher Publisher, frontendURL, timezone string) (*ReminderScheduler, error) {
	location, err := time.LoadLocation(timezone)
	if err != nil {
		return nil, err
	}
	return &ReminderScheduler{db: db, publisher: publisher, frontendURL: strings.TrimRight(frontendURL, "/"), location: location}, nil
}

func (s *ReminderScheduler) Run(ctx context.Context) {
	s.runOnce(ctx)
	for {
		timer := time.NewTimer(time.Until(nextReminderRun(time.Now(), s.location)))
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
			s.runOnce(ctx)
		}
	}
}

func (s *ReminderScheduler) runOnce(ctx context.Context) {
	if err := s.process(ctx, time.Now().In(s.location)); err != nil && ctx.Err() == nil {
		slog.Error("deadline reminder processing failed", "error", err)
	}
}

func (s *ReminderScheduler) process(ctx context.Context, now time.Time) error {
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, s.location)
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	rows, err := tx.Query(ctx, `
		SELECT t.id::text, t.title, t.due_date, employee.email, manager.email
		FROM tasks t
		JOIN idps i ON i.id=t.idp_id
		JOIN users employee ON employee.id=i.employee_id AND employee.is_active=true
		JOIN users manager ON manager.id=i.manager_id AND manager.is_active=true
		WHERE t.deleted_at IS NULL AND i.archived_at IS NULL AND i.status='active'
			AND t.status NOT IN ('completed','cancelled')
			AND (t.due_date=$1::date + 3 OR t.due_date<$1::date)
		FOR UPDATE OF t SKIP LOCKED
	`, today)
	if err != nil {
		return err
	}
	type reminder struct {
		id, title, employeeEmail, managerEmail string
		dueDate                                time.Time
	}
	var items []reminder
	for rows.Next() {
		var item reminder
		if err := rows.Scan(&item.id, &item.title, &item.dueDate, &item.employeeEmail, &item.managerEmail); err != nil {
			rows.Close()
			return err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()

	for _, item := range items {
		kind := "deadline_soon"
		if item.dueDate.Before(today) {
			kind = "overdue"
		}
		tag, err := tx.Exec(ctx, `INSERT INTO notification_reminders (task_id,kind,due_date)
			VALUES ($1,$2,$3) ON CONFLICT DO NOTHING`, item.id, kind, item.dueDate)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			continue
		}
		recipients := []string{item.employeeEmail}
		if kind == "overdue" && item.managerEmail != item.employeeEmail {
			recipients = append(recipients, item.managerEmail)
		}
		for _, email := range recipients {
			if err := s.publisher.EnqueueTx(ctx, tx, Job{
				To: []string{email}, Template: TaskDeadlineTemplate,
				Data: map[string]string{
					"kind": kind, "task_title": item.title,
					"due_date": item.dueDate.Format(time.DateOnly), "plans_url": s.frontendURL + "/plans",
				},
			}); err != nil {
				return err
			}
		}
	}
	return tx.Commit(ctx)
}

func nextReminderRun(now time.Time, location *time.Location) time.Time {
	local := now.In(location)
	next := time.Date(local.Year(), local.Month(), local.Day(), 9, 0, 0, 0, location)
	if !next.After(local) {
		next = next.AddDate(0, 0, 1)
	}
	return next
}
