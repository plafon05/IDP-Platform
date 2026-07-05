package notification

import "testing"

func TestContentForJob(t *testing.T) {
	content, ok := contentForJob(Job{
		UserID: "user-id", Template: CommentCreatedTemplate,
		Data: map[string]string{
			"author_name": "Иван Иванов", "entity_title": "План развития", "plans_url": "https://idp.test/plans?id=1",
		},
	})
	if !ok || content.title != "Новый комментарий" || content.actionURL != "/plans?id=1" {
		t.Fatalf("unexpected content: %#v, ok=%v", content, ok)
	}
}

func TestContentForJobSkipsServiceEmails(t *testing.T) {
	for _, template := range []string{PasswordResetTemplate, WelcomeTemplate} {
		if _, ok := contentForJob(Job{UserID: "user-id", Template: template}); ok {
			t.Fatalf("template %q must not create an in-app notification", template)
		}
	}
}
