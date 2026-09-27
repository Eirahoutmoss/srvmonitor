package config

import (
	"os"
	"path/filepath"
	"testing"

	"srvmon/internal/alert"
)

const good = `{
  // sunucu adı
  "server_name": "PROD-01",
  "poll_seconds": 10,
  "web": { "listen": ":8085" },
  "databases": [ { "name": "PROD", "type": "oracle", "dsn": "oracle://u:p@h:1521/orcl" } ],
  "rules": [
    { "id": "cpu", "desc": "CPU", "key": "cpu.percent", "compare": "gt",
      "warn": 80, "crit": 90, "for": "2m", "cooldown": "5m", "channels": ["email","sms"] }
  ]
}`

func write(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLoadGood(t *testing.T) {
	c, err := Load(write(t, good))
	if err != nil {
		t.Fatal(err)
	}
	if c.ServerName != "PROD-01" || c.PollInterval().Seconds() != 10 {
		t.Fatalf("unexpected config: %+v", c)
	}
	rules, err := c.EngineRules()
	if err != nil || len(rules) != 1 {
		t.Fatalf("rules err=%v n=%d", err, len(rules))
	}
	if rules[0].Compare != alert.GT || rules[0].For.String() != "2m0s" {
		t.Fatalf("rule parse wrong: %+v", rules[0])
	}
	if ch := c.ChannelsByRule()["cpu"]; len(ch) != 2 {
		t.Fatalf("channels = %v", ch)
	}
}

func TestRejectBadCompare(t *testing.T) {
	body := `{"web":{"listen":":1"},"rules":[{"id":"x","key":"cpu.percent","compare":"eq","warn":1,"crit":2}]}`
	if _, err := Load(write(t, body)); err == nil {
		t.Fatal("expected error for bad comparator")
	}
}

func TestRejectMissingListen(t *testing.T) {
	if _, err := Load(write(t, `{"rules":[]}`)); err == nil {
		t.Fatal("expected error for missing web.listen")
	}
}

func TestRejectDuplicateRuleID(t *testing.T) {
	body := `{"web":{"listen":":1"},"rules":[
		{"id":"x","key":"cpu.percent","compare":"gt","warn":1,"crit":2},
		{"id":"x","key":"mem.percent","compare":"gt","warn":1,"crit":2}]}`
	if _, err := Load(write(t, body)); err == nil {
		t.Fatal("expected error for duplicate rule id")
	}
}

func TestRejectBadDuration(t *testing.T) {
	body := `{"web":{"listen":":1"},"rules":[{"id":"x","key":"cpu.percent","compare":"gt","warn":1,"crit":2,"for":"soon"}]}`
	if _, err := Load(write(t, body)); err == nil {
		t.Fatal("expected error for bad duration")
	}
}

func TestUnknownFieldRejected(t *testing.T) {
	if _, err := Load(write(t, `{"web":{"listen":":1"},"nope":true}`)); err == nil {
		t.Fatal("expected error for unknown field")
	}
}
