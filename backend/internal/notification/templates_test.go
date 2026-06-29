package notification

import (
	"strings"
	"testing"
)

func TestRenderPasswordResetEscapesURL(t *testing.T) {
	message, err := Render(Job{Template: PasswordResetTemplate, Data: map[string]string{"reset_url": "https://example.test/reset?a=1&b=2"}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(message.HTML, "Сброс пароля") || strings.Contains(message.HTML, "a=1&b=2") {
		t.Fatal("HTML template was not rendered or escaped safely")
	}
}

func TestRenderIDPStatusIncludesReason(t *testing.T) {
	message, err := Render(Job{Template: IDPStatusTemplate, Data: map[string]string{
		"idp_title": "Рост до senior", "status": "cancelled",
		"idp_url": "https://example.test/plans", "reason": "Изменились цели",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(message.Text, "отменён") || !strings.Contains(message.Text, "Изменились цели") {
		t.Fatal("status or cancellation reason is missing")
	}
}

func TestRenderIDPStatusRejectsUnknownStatus(t *testing.T) {
	_, err := Render(Job{Template: IDPStatusTemplate, Data: map[string]string{
		"idp_title": "Plan", "status": "draft", "idp_url": "https://example.test/plans",
	}})
	if err == nil {
		t.Fatal("unknown status must be rejected")
	}
}

func TestRenderTaskReview(t *testing.T) {
	message, err := Render(Job{Template: TaskReviewTemplate, Data: map[string]string{
		"task_title": "Курс Go", "rating": "met", "comment": "Отлично",
		"plans_url": "https://example.test/plans",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(message.Text, "Выполнено") || !strings.Contains(message.Text, "Отлично") {
		t.Fatal("manager review data is missing")
	}
}

func TestRenderTaskChanged(t *testing.T) {
	message, err := Render(Job{Template: TaskChangedTemplate, Data: map[string]string{
		"event": "created", "task_title": "Курс Go", "due_date": "2026-06-30",
		"plans_url": "https://example.test/plans",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(message.Text, "назначена") || !strings.Contains(message.Text, "2026-06-30") {
		t.Fatal("task change data is missing")
	}
}

func TestRenderCommentCreatedEscapesContent(t *testing.T) {
	message, err := Render(Job{Template: CommentCreatedTemplate, Data: map[string]string{
		"author_name": "Иван Иванов", "entity_type": "task", "entity_title": "Курс Go",
		"excerpt": "Готово <script>alert(1)</script>", "plans_url": "https://example.test/plans",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(message.HTML, "<script>") || !strings.Contains(message.Text, "Готово") {
		t.Fatal("comment must be rendered and escaped")
	}
}

func TestRenderTaskDeadline(t *testing.T) {
	message, err := Render(Job{Template: TaskDeadlineTemplate, Data: map[string]string{
		"kind": "overdue", "task_title": "Курс Go", "due_date": "2026-06-28",
		"plans_url": "https://example.test/plans",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(message.Subject, "просрочена") || !strings.Contains(message.Text, "2026-06-28") {
		t.Fatal("deadline data is missing")
	}
}

func TestRenderWelcomeWithoutPassword(t *testing.T) {
	message, err := Render(Job{Template: WelcomeTemplate, Data: map[string]string{"login_url": "https://example.test"}})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.ToLower(message.Text), "password") || !strings.Contains(message.Text, "https://example.test") {
		t.Fatal("welcome message must contain login URL and no password")
	}
}

func TestRenderAddsUnsubscribeFooter(t *testing.T) {
	message, err := Render(Job{Template: TaskDeadlineTemplate, Data: map[string]string{
		"kind": "overdue", "task_title": "Task", "due_date": "2026-06-28",
		"plans_url": "https://example.test/plans", "unsubscribe_url": "https://example.test/unsubscribe?token=a&b=c",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(message.Text, "Отписаться") || strings.Contains(message.HTML, "token=a&b=c") {
		t.Fatal("unsubscribe footer is missing or not escaped")
	}
}
