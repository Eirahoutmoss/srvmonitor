// Package config loads and validates the srvmon configuration file (JSON).
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"srvmon/internal/alert"
)

// Config is the whole application configuration.
type Config struct {
	ServerName      string           `json:"server_name"`
	PollSeconds     int              `json:"poll_seconds"`
	DeepSeconds     int              `json:"deep_seconds"`         // servis + olay taraması aralığı
	EventWindowMins int              `json:"event_window_minutes"` // olay günlüğü geriye bakış (dk)
	EventMax        int              `json:"event_max"`            // en fazla olay kaydı
	HistorySamples  int              `json:"history_samples"`
	Web             WebConfig        `json:"web"`
	Databases       []DatabaseConfig `json:"databases"`
	Watch           WatchConfig      `json:"watch"`
	Rules           []RuleConfig     `json:"rules"`
	Email           EmailConfig      `json:"email"`
	SMS             SMSConfig        `json:"sms"`
	Digest          DigestConfig     `json:"digest"`
}

// WatchConfig names the specific services, processes and health endpoints to
// monitor — including the operator's own application.
type WatchConfig struct {
	Services  []string         `json:"services"`  // Windows servis adları (ör. "MSSQLSERVER")
	Processes []string         `json:"processes"` // süreç görüntü adları (ör. "MyApp.exe")
	Endpoints []EndpointConfig `json:"endpoints"` // HTTP/TCP sağlık uçları
}

// EndpointConfig is one health probe for a custom application. Set URL for an
// HTTP check or TCP for a plain connection check.
type EndpointConfig struct {
	Name         string `json:"name"`
	URL          string `json:"url"`           // http(s) sağlık adresi
	TCP          string `json:"tcp"`           // host:port
	ExpectStatus int    `json:"expect_status"` // 0 => herhangi bir 2xx
}

// DigestConfig configures the periodic status e-mail. It reuses the SMTP
// settings from EmailConfig. Set either EveryHours or DailyAt.
type DigestConfig struct {
	Enabled    bool     `json:"enabled"`
	EveryHours int      `json:"every_hours"` // ör. 8 saatte bir
	DailyAt    string   `json:"daily_at"`    // "HH:MM" yerel saat; ayarlıysa EveryHours yerine geçer
	To         []string `json:"to"`          // boşsa email.to kullanılır
}

// Recipients returns the digest recipients, falling back to the alarm list.
func (d DigestConfig) Recipients(fallback []string) []string {
	if len(d.To) > 0 {
		return d.To
	}
	return fallback
}

// DeepInterval returns the service/event scan period, defaulting to 60s.
func (c Config) DeepInterval() time.Duration {
	if c.DeepSeconds <= 0 {
		return 60 * time.Second
	}
	return time.Duration(c.DeepSeconds) * time.Second
}

// EventWindow returns how far back to read the Event Log, defaulting to 30m.
func (c Config) EventWindow() time.Duration {
	if c.EventWindowMins <= 0 {
		return 30 * time.Minute
	}
	return time.Duration(c.EventWindowMins) * time.Minute
}

// EventLimit returns the maximum number of events to read, defaulting to 50.
func (c Config) EventLimit() int {
	if c.EventMax <= 0 {
		return 50
	}
	return c.EventMax
}

// WebConfig configures the dashboard HTTP server.
type WebConfig struct {
	Listen string `json:"listen"` // e.g. ":8085"
	Token  string `json:"token"`  // optional ?token= gate for the API
}

// DatabaseConfig is one database to monitor. Type is oracle | postgres | mysql
// (mariadb is accepted as an alias of mysql). DSN examples:
//
//	oracle://user:pass@host:1521/service
//	postgres://user:pass@host:5432/dbname?sslmode=disable
//	user:pass@tcp(host:3306)/dbname
type DatabaseConfig struct {
	Name string `json:"name"`
	Type string `json:"type"`
	DSN  string `json:"dsn"`
}

// RuleConfig is the on-disk form of an alert rule (durations as strings).
type RuleConfig struct {
	ID       string   `json:"id"`
	Desc     string   `json:"desc"`
	Key      string   `json:"key"`
	Compare  string   `json:"compare"`
	Warn     float64  `json:"warn"`
	Crit     float64  `json:"crit"`
	For      string   `json:"for"`
	Cooldown string   `json:"cooldown"`
	Channels []string `json:"channels"` // "email", "sms"
}

// EmailConfig configures SMTP delivery.
type EmailConfig struct {
	Enabled  bool     `json:"enabled"`
	Host     string   `json:"host"`
	Port     int      `json:"port"`
	Username string   `json:"username"`
	Password string   `json:"password"`
	From     string   `json:"from"`
	To       []string `json:"to"`
	STARTTLS bool     `json:"starttls"`
}

// SMSConfig configures a generic HTTP SMS gateway. The URL, headers and body
// may contain {message}, {to}, {severity} placeholders so any provider
// (Twilio, Netgsm, İletimerkezi, ...) can be wired without code changes.
type SMSConfig struct {
	Enabled bool              `json:"enabled"`
	Method  string            `json:"method"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers"`
	Body    string            `json:"body"`
	To      []string          `json:"to"`
}

// Rules converts the on-disk rules into engine rules, parsing durations.
func (c Config) EngineRules() ([]alert.Rule, error) {
	out := make([]alert.Rule, 0, len(c.Rules))
	for _, r := range c.Rules {
		cmp := alert.Comparator(r.Compare)
		if cmp != alert.GT && cmp != alert.LT {
			return nil, fmt.Errorf("kural %q: compare 'gt' veya 'lt' olmalı, %q verildi", r.ID, r.Compare)
		}
		forD, err := parseDur(r.For)
		if err != nil {
			return nil, fmt.Errorf("kural %q: geçersiz 'for' süresi: %w", r.ID, err)
		}
		cd, err := parseDur(r.Cooldown)
		if err != nil {
			return nil, fmt.Errorf("kural %q: geçersiz 'cooldown' süresi: %w", r.ID, err)
		}
		out = append(out, alert.Rule{
			ID: r.ID, Desc: r.Desc, Key: r.Key, Compare: cmp,
			Warn: r.Warn, Crit: r.Crit, For: forD, Cooldown: cd,
		})
	}
	return out, nil
}

// ChannelsByRule maps rule ID to its notification channels.
func (c Config) ChannelsByRule() map[string][]string {
	m := map[string][]string{}
	for _, r := range c.Rules {
		m[r.ID] = r.Channels
	}
	return m
}

func parseDur(s string) (time.Duration, error) {
	if s == "" {
		return 0, nil
	}
	return time.ParseDuration(s)
}

// PollInterval returns the poll period, defaulting to 15s.
func (c Config) PollInterval() time.Duration {
	if c.PollSeconds <= 0 {
		return 15 * time.Second
	}
	return time.Duration(c.PollSeconds) * time.Second
}

// Load reads and validates a config file.
func Load(path string) (*Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Parse(b)
}

// Parse validates a configuration from raw JSON bytes (line // comments ok).
func Parse(b []byte) (*Config, error) {
	var c Config
	dec := json.NewDecoder(newCommentStripper(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&c); err != nil {
		return nil, fmt.Errorf("config ayrıştırılamadı: %w", err)
	}
	if err := c.validate(); err != nil {
		return nil, err
	}
	return &c, nil
}

// Save writes the configuration back to disk as pretty JSON.
func (c *Config) Save(path string) error {
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o600)
}

func (c Config) validate() error {
	if c.Web.Listen == "" {
		return fmt.Errorf("web.listen boş olamaz (örn. \":8085\")")
	}
	seen := map[string]bool{}
	for _, o := range c.Databases {
		if o.Name == "" || o.DSN == "" || o.Type == "" {
			return fmt.Errorf("databases girişlerinde name, type ve dsn zorunlu")
		}
		switch strings.ToLower(o.Type) {
		case "oracle", "postgres", "mysql", "mariadb":
		default:
			return fmt.Errorf("veritabanı %q: type oracle|postgres|mysql olmalı, %q verildi", o.Name, o.Type)
		}
		if seen[o.Name] {
			return fmt.Errorf("veritabanı adı tekrar ediyor: %q", o.Name)
		}
		seen[o.Name] = true
	}
	ids := map[string]bool{}
	for _, r := range c.Rules {
		if r.ID == "" || r.Key == "" {
			return fmt.Errorf("her kuralda id ve key zorunlu")
		}
		if ids[r.ID] {
			return fmt.Errorf("kural id tekrar ediyor: %q", r.ID)
		}
		ids[r.ID] = true
	}
	if _, err := c.EngineRules(); err != nil {
		return err
	}
	if c.Digest.Enabled {
		if c.Email.Host == "" {
			return fmt.Errorf("digest etkin ama email.host boş (SMTP ayarları gerekli)")
		}
		if _, err := c.Digest.NextDigest(time.Now()); err != nil {
			return err
		}
	}
	return nil
}
