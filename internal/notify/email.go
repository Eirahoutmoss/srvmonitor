package notify

import (
	"fmt"
	"net/smtp"
	"strings"

	"srvmon/internal/config"
)

// Email sends notifications over SMTP using the standard library.
type Email struct {
	cfg config.EmailConfig
	// send is swappable in tests; nil uses net/smtp.
	send func(addr string, a smtp.Auth, from string, to []string, msg []byte) error
}

// NewEmail builds an Email channel, or nil when disabled.
func NewEmail(cfg config.EmailConfig) *Email {
	if !cfg.Enabled {
		return nil
	}
	return &Email{cfg: cfg, send: smtp.SendMail}
}

// Name implements Channel.
func (e *Email) Name() string { return "email" }

// Build renders the raw RFC 5322 message bytes for a notification.
func (e *Email) Build(m Message) []byte {
	var b strings.Builder
	fmt.Fprintf(&b, "From: %s\r\n", e.cfg.From)
	fmt.Fprintf(&b, "To: %s\r\n", strings.Join(e.cfg.To, ", "))
	fmt.Fprintf(&b, "Subject: %s\r\n", m.Subject)
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/plain; charset=UTF-8\r\n\r\n")
	b.WriteString(m.Body)
	return []byte(b.String())
}

// Send implements Channel.
func (e *Email) Send(m Message) error {
	if len(e.cfg.To) == 0 {
		return fmt.Errorf("email alıcı listesi boş")
	}
	addr := fmt.Sprintf("%s:%d", e.cfg.Host, e.cfg.Port)
	var auth smtp.Auth
	if e.cfg.Username != "" {
		auth = smtp.PlainAuth("", e.cfg.Username, e.cfg.Password, e.cfg.Host)
	}
	return e.send(addr, auth, e.cfg.From, e.cfg.To, e.Build(m))
}
