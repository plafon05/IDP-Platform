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
