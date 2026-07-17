package notification

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"mime"
	"mime/multipart"
	"net"
	"net/mail"
	"net/smtp"
	"net/textproto"
	"strings"
	"time"

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
	connection, err := net.DialTimeout("tcp", address, 15*time.Second)
	if err != nil {
		return err
	}
	defer connection.Close()
	if err := connection.SetDeadline(time.Now().Add(30 * time.Second)); err != nil {
		return err
	}
	client, err := smtp.NewClient(connection, s.cfg.SMTPHost)
	if err != nil {
		return err
	}
	defer client.Quit()
	if ok, _ := client.Extension("STARTTLS"); ok {
		if err := client.StartTLS(&tls.Config{ServerName: s.cfg.SMTPHost, MinVersion: tls.VersionTLS12}); err != nil {
			return err
		}
	} else if s.cfg.AppEnv == "production" {
		return fmt.Errorf("SMTP server does not support STARTTLS")
	}
	if authentication != nil {
		if err := client.Auth(authentication); err != nil {
			return err
		}
	}
	if err := client.Mail(s.cfg.SMTPFromEmail); err != nil {
		return err
	}
	for _, recipient := range recipients {
		if err := client.Rcpt(recipient); err != nil {
			return err
		}
	}
	smtpWriter, err := client.Data()
	if err != nil {
		return err
	}
	if _, err := smtpWriter.Write(body.Bytes()); err != nil {
		return err
	}
	return smtpWriter.Close()
}
