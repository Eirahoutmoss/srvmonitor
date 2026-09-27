package notify

import (
	"fmt"
	"net/smtp"
	"strings"

	"srvmon/internal/config"
)

// SendHTML delivers an HTML e-mail over SMTP using the given settings. It is
// used by the periodic status digest and is independent of the alarm channel's
// enabled flag.
func SendHTML(cfg config.EmailConfig, to []string, subject, htmlBody string) error {
	if cfg.Host == "" {
		return fmt.Errorf("email.host boş")
	}
	if len(to) == 0 {
		return fmt.Errorf("alıcı listesi boş")
	}
	var b strings.Builder
	fmt.Fprintf(&b, "From: %s\r\n", cfg.From)
	fmt.Fprintf(&b, "To: %s\r\n", strings.Join(to, ", "))
	fmt.Fprintf(&b, "Subject: %s\r\n", subject)
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/html; charset=UTF-8\r\n\r\n")
	b.WriteString(htmlBody)

	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	var auth smtp.Auth
	if cfg.Username != "" {
		auth = smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host)
	}
	return smtp.SendMail(addr, auth, cfg.From, to, []byte(b.String()))
}
