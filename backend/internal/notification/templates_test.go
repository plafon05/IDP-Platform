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
