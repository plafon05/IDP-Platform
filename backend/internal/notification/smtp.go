package notification

import (
	"bytes"
	"fmt"
	"mime"
	"mime/multipart"
	"net/mail"
	"net/smtp"
	"net/textproto"
	"strings"

	"idp-platform/backend/internal/config"
)

type SMTPSender struct{ cfg config.Config }

func NewSMTPSender(cfg config.Config) *SMTPSender { return &SMTPSender{cfg: cfg} }

func (s *SMTPSender) Send(to []string, message RenderedMessage) error {
	if len(to) == 0 {
		return fmt.Errorf("email recipient is required")
	}
	from := mail.Address{Name: s.cfg.SMTPFromName, Address: s.cfg.SMTPFromEmail}
	recipients := make([]string, 0, len(to))
	for _, raw := range to {
		address, err := mail.ParseAddress(raw)
		if err != nil {
			return fmt.Errorf("invalid email recipient: %w", err)
		}
		recipients = append(recipients, address.Address)
	}
	var body bytes.Buffer
	body.WriteString("From: " + from.String() + "\r\n")
	body.WriteString("To: " + strings.Join(recipients, ", ") + "\r\n")
	body.WriteString("Subject: " + mime.QEncoding.Encode("UTF-8", message.Subject) + "\r\n")
	body.WriteString("MIME-Version: 1.0\r\n")
	writer := multipart.NewWriter(&body)
	body.WriteString("Content-Type: multipart/alternative; boundary=" + writer.Boundary() + "\r\n\r\n")
	textPart, err := writer.CreatePart(textproto.MIMEHeader{"Content-Type": {"text/plain; charset=UTF-8"}})
	if err != nil {
		return err
	}
	if _, err := textPart.Write([]byte(message.Text)); err != nil {
		return err
	}
	htmlPart, err := writer.CreatePart(textproto.MIMEHeader{"Content-Type": {"text/html; charset=UTF-8"}})
	if err != nil {
		return err
	}
	if _, err := htmlPart.Write([]byte(message.HTML)); err != nil {
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}

	address := fmt.Sprintf("%s:%d", s.cfg.SMTPHost, s.cfg.SMTPPort)
	var authentication smtp.Auth
	if s.cfg.SMTPUsername != "" {
		authentication = smtp.PlainAuth("", s.cfg.SMTPUsername, s.cfg.SMTPPassword, s.cfg.SMTPHost)
	}
	return smtp.SendMail(address, authentication, s.cfg.SMTPFromEmail, recipients, body.Bytes())
}
