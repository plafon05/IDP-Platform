package notification

import (
	"bytes"
	"errors"
	"html/template"
	"strings"
)

const (
	PasswordResetTemplate = "password_reset"
	IDPStatusTemplate     = "idp_status_changed"
	TaskReviewTemplate    = "task_manager_review"
)

type RenderedMessage struct {
	Subject string
	Text    string
	HTML    string
}

func Render(job Job) (RenderedMessage, error) {
	switch job.Template {
	case PasswordResetTemplate:
		return renderPasswordReset(job.Data)
	case IDPStatusTemplate:
		return renderIDPStatus(job.Data)
	case TaskReviewTemplate:
		return renderTaskReview(job.Data)
	default:
		return RenderedMessage{}, errors.New("unknown notification template")
	}
}

func renderTaskReview(data map[string]string) (RenderedMessage, error) {
	ratingLabels := map[string]string{
		"met": "Выполнено", "partially_met": "Частично выполнено", "not_met": "Не выполнено",
	}
	ratingLabel := ""
	if data["rating"] != "" {
		var ok bool
		ratingLabel, ok = ratingLabels[data["rating"]]
		if !ok {
			return RenderedMessage{}, errors.New("unknown manager rating")
		}
	}
	if data["task_title"] == "" || data["plans_url"] == "" || (ratingLabel == "" && strings.TrimSpace(data["comment"]) == "") {
		return RenderedMessage{}, errors.New("invalid task review notification data")
	}
	view := map[string]string{
		"task_title": data["task_title"], "plans_url": data["plans_url"],
		"rating_label": ratingLabel, "comment": data["comment"],
	}
	const body = `<!doctype html><html><body style="margin:0;background:#f8fafc;font-family:Arial,sans-serif;color:#1e293b"><div style="max-width:600px;margin:0 auto;padding:32px 20px"><div style="background:#fff;border:1px solid #e2e8f0;padding:28px"><h1 style="font-size:22px;margin:0 0 16px">Оценка задачи</h1><p>Руководитель добавил оценку к задаче «{{.task_title}}».</p>{{if .rating_label}}<p><strong>Результат:</strong> {{.rating_label}}</p>{{end}}{{if .comment}}<p><strong>Комментарий:</strong> {{.comment}}</p>{{end}}<p><a href="{{.plans_url}}" style="display:inline-block;padding:11px 16px;background:#2563eb;color:#fff;text-decoration:none">Открыть ИПР</a></p></div></div></body></html>`
	tmpl, err := template.New("task-review").Parse(body)
	if err != nil {
		return RenderedMessage{}, err
	}
	var html bytes.Buffer
	if err := tmpl.Execute(&html, view); err != nil {
		return RenderedMessage{}, err
	}
	text := "Руководитель добавил оценку к задаче «" + data["task_title"] + "»."
	if ratingLabel != "" {
		text += "\nРезультат: " + ratingLabel
	}
	if comment := strings.TrimSpace(data["comment"]); comment != "" {
		text += "\nКомментарий: " + comment
	}
	return RenderedMessage{
		Subject: "Оценка задачи: " + data["task_title"],
		Text:    text + "\n" + data["plans_url"],
		HTML:    html.String(),
	}, nil
}

func renderIDPStatus(data map[string]string) (RenderedMessage, error) {
	statusLabels := map[string]string{
		"active": "активирован", "completed": "завершён", "cancelled": "отменён",
	}
	statusLabel, ok := statusLabels[data["status"]]
	if !ok || data["idp_title"] == "" || data["idp_url"] == "" {
		return RenderedMessage{}, errors.New("invalid idp status notification data")
	}
	view := map[string]string{
		"first_name": data["first_name"], "idp_title": data["idp_title"],
		"idp_url": data["idp_url"], "reason": data["reason"], "status_label": statusLabel,
	}
	const body = `<!doctype html><html><body style="margin:0;background:#f8fafc;font-family:Arial,sans-serif;color:#1e293b"><div style="max-width:600px;margin:0 auto;padding:32px 20px"><div style="background:#fff;border:1px solid #e2e8f0;padding:28px"><h1 style="font-size:22px;margin:0 0 16px">Статус ИПР изменён</h1><p>ИПР «{{.idp_title}}» {{.status_label}}.</p>{{if .reason}}<p><strong>Причина:</strong> {{.reason}}</p>{{end}}<p><a href="{{.idp_url}}" style="display:inline-block;padding:11px 16px;background:#2563eb;color:#fff;text-decoration:none">Открыть ИПР</a></p></div></div></body></html>`
	tmpl, err := template.New("idp-status").Parse(body)
	if err != nil {
		return RenderedMessage{}, err
	}
	var html bytes.Buffer
	if err := tmpl.Execute(&html, view); err != nil {
		return RenderedMessage{}, err
	}
	text := "ИПР «" + data["idp_title"] + "» " + statusLabel + "."
	if reason := strings.TrimSpace(data["reason"]); reason != "" {
		text += "\nПричина: " + reason
	}
	return RenderedMessage{
		Subject: "Изменён статус ИПР: " + data["idp_title"],
		Text:    text + "\n" + data["idp_url"],
		HTML:    html.String(),
	}, nil
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
