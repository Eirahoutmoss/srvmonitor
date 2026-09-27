package notify

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"srvmon/internal/config"
)

// SMS sends notifications through a generic HTTP gateway. The URL, headers and
// body may contain {message}, {to} and {severity} placeholders, so any
// provider can be configured without code changes.
type SMS struct {
	cfg    config.SMSConfig
	client *http.Client
}

// NewSMS builds an SMS channel, or nil when disabled.
func NewSMS(cfg config.SMSConfig) *SMS {
	if !cfg.Enabled {
		return nil
	}
	method := strings.ToUpper(strings.TrimSpace(cfg.Method))
	if method == "" {
		method = http.MethodPost
	}
	cfg.Method = method
	return &SMS{cfg: cfg, client: &http.Client{Timeout: 15 * time.Second}}
}

// Name implements Channel.
func (s *SMS) Name() string { return "sms" }

// Placeholders builds the substitution map for one message and recipient.
func placeholders(m Message, to string) *strings.Replacer {
	return strings.NewReplacer(
		"{message}", m.Subject+" — "+strings.ReplaceAll(m.Body, "\n", " "),
		"{severity}", m.Severity,
		"{to}", to,
	)
}

// buildRequest renders the HTTP request for one recipient (exposed for tests).
func (s *SMS) buildRequest(m Message, to string) (*http.Request, error) {
	rep := placeholders(m, to)
	url := rep.Replace(s.cfg.URL)
	body := rep.Replace(s.cfg.Body)
	req, err := http.NewRequest(s.cfg.Method, url, bytes.NewBufferString(body))
	if err != nil {
		return nil, err
	}
	for k, v := range s.cfg.Headers {
		req.Header.Set(k, rep.Replace(v))
	}
	return req, nil
}

// Send implements Channel, delivering to every configured recipient.
func (s *SMS) Send(m Message) error {
	if len(s.cfg.To) == 0 {
		return fmt.Errorf("sms alıcı listesi boş")
	}
	var firstErr error
	for _, to := range s.cfg.To {
		req, err := s.buildRequest(m, to)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		resp, err := s.client.Do(req)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		resp.Body.Close()
		if resp.StatusCode >= 300 {
			if firstErr == nil {
				firstErr = fmt.Errorf("%s için HTTP %d: %s", to, resp.StatusCode, strings.TrimSpace(string(b)))
			}
		}
	}
	return firstErr
}
