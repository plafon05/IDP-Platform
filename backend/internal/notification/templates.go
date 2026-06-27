package notification

import (
	"bytes"
	"errors"
	"html/template"
)

const PasswordResetTemplate = "password_reset"

type RenderedMessage struct {
	Subject string
	Text    string
	HTML    string
}

func Render(job Job) (RenderedMessage, error) {
	switch job.Template {
	case PasswordResetTemplate:
		return renderPasswordReset(job.Data)
	default:
		return RenderedMessage{}, errors.New("unknown notification template")
	}
}

func renderPasswordReset(data map[string]string) (RenderedMessage, error) {
	resetURL := data["reset_url"]
	if resetURL == "" {
		return RenderedMessage{}, errors.New("reset_url is required")
	}
	const body = `<!doctype html><html><body style="margin:0;background:#f8fafc;font-family:Arial,sans-serif;color:#1e293b"><div style="max-width:600px;margin:0 auto;padding:32px 20px"><div style="background:#fff;border:1px solid #e2e8f0;padding:28px"><h1 style="font-size:22px;margin:0 0 16px">Сброс пароля</h1><p>Получен запрос на смену пароля в IDP Platform.</p><p><a href="{{.reset_url}}" style="display:inline-block;padding:11px 16px;background:#2563eb;color:#fff;text-decoration:none">Создать новый пароль</a></p><p style="color:#64748b;font-size:13px">Ссылка действует 30 минут. Если вы не запрашивали сброс, проигнорируйте письмо.</p></div></div></body></html>`
	tmpl, err := template.New("password-reset").Parse(body)
	if err != nil {
		return RenderedMessage{}, err
	}
	var html bytes.Buffer
	if err := tmpl.Execute(&html, data); err != nil {
		return RenderedMessage{}, err
	}
	return RenderedMessage{
		Subject: "Сброс пароля в IDP Platform",
		Text:    "Создайте новый пароль: " + resetURL + "\nСсылка действует 30 минут.",
		HTML:    html.String(),
	}, nil
}
