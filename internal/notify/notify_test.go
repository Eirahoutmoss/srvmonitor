package notify

import (
	"net/smtp"
	"strings"
	"testing"
	"time"

	"srvmon/internal/alert"
	"srvmon/internal/config"
)

func TestFromEventBreach(t *testing.T) {
	ev := alert.Event{RuleID: "cpu", Desc: "CPU yüksek", Key: "cpu.percent",
		Level: alert.Crit, Previous: alert.Warn, Value: 95, Threshold: 90, Time: time.Now()}
	m := FromEvent("PROD-01", ev)
	if m.Severity != "KRİTİK" {
		t.Errorf("severity = %q", m.Severity)
	}
	if !strings.Contains(m.Subject, "PROD-01") || !strings.Contains(m.Subject, "KRİTİK") {
		t.Errorf("subject = %q", m.Subject)
	}
	if !strings.Contains(m.Body, "cpu.percent") {
		t.Errorf("body missing metric: %q", m.Body)
	}
}

func TestFromEventRecovery(t *testing.T) {
	ev := alert.Event{RuleID: "cpu", Desc: "CPU yüksek", Key: "cpu.percent",
		Level: alert.OK, Previous: alert.Crit, Value: 10, Time: time.Now()}
	m := FromEvent("PROD-01", ev)
	if !strings.Contains(m.Subject, "NORMALE DÖNDÜ") {
		t.Errorf("recovery subject = %q", m.Subject)
	}
}

func TestEmailBuild(t *testing.T) {
	e := &Email{cfg: config.EmailConfig{From: "a@x.com", To: []string{"b@y.com", "c@y.com"}}}
	raw := string(e.Build(Message{Subject: "Konu", Body: "Gövde"}))
	for _, want := range []string{"From: a@x.com", "To: b@y.com, c@y.com", "Subject: Konu", "charset=UTF-8", "Gövde"} {
		if !strings.Contains(raw, want) {
			t.Errorf("email missing %q in:\n%s", want, raw)
		}
	}
}

func TestEmailSendUsesConfig(t *testing.T) {
	var gotAddr, gotFrom string
	var gotTo []string
	e := &Email{
		cfg: config.EmailConfig{Host: "smtp.x.com", Port: 587, From: "a@x.com", To: []string{"b@y.com"}},
		send: func(addr string, _ smtp.Auth, from string, to []string, _ []byte) error {
			gotAddr, gotFrom, gotTo = addr, from, to
			return nil
		},
	}
	if err := e.Send(Message{Subject: "s", Body: "b"}); err != nil {
		t.Fatal(err)
	}
	if gotAddr != "smtp.x.com:587" || gotFrom != "a@x.com" || len(gotTo) != 1 {
		t.Fatalf("send args wrong: %s %s %v", gotAddr, gotFrom, gotTo)
	}
}

func TestSMSTemplate(t *testing.T) {
	s := &SMS{cfg: config.SMSConfig{
		Method: "POST", URL: "https://gw/send?to={to}",
		Headers: map[string]string{"X-Sev": "{severity}"},
		Body:    `{"to":"{to}","text":"{message}"}`,
		To:      []string{"+9055500"},
	}}
	req, err := s.buildRequest(Message{Severity: "KRİTİK", Subject: "Konu", Body: "Gövde"}, "+9055500")
	if err != nil {
		t.Fatal(err)
	}
	if req.URL.String() != "https://gw/send?to=+9055500" && !strings.Contains(req.URL.String(), "9055500") {
		t.Errorf("url = %s", req.URL.String())
	}
	if req.Header.Get("X-Sev") != "KRİTİK" {
		t.Errorf("header not rendered: %q", req.Header.Get("X-Sev"))
	}
}

func TestDisabledChannelsAreNil(t *testing.T) {
	if NewEmail(config.EmailConfig{Enabled: false}) != nil {
		t.Error("disabled email should be nil")
	}
	if NewSMS(config.SMSConfig{Enabled: false}) != nil {
		t.Error("disabled sms should be nil")
	}
}
