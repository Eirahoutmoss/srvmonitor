package collect

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql" // registers "mysql"
	_ "github.com/jackc/pgx/v5/stdlib" // registers "pgx"
	_ "github.com/sijms/go-ora/v2"     // registers "oracle"

	"srvmon/internal/config"
	"srvmon/internal/model"
)

// Supported engine types.
const (
	Oracle   = "oracle"
	Postgres = "postgres"
	MySQL    = "mysql"
)

// driverName maps an engine type to its database/sql driver.
func driverName(typ string) (string, error) {
	switch strings.ToLower(typ) {
	case Oracle:
		return "oracle", nil
	case Postgres:
		return "pgx", nil
	case MySQL, "mariadb":
		return "mysql", nil
	default:
		return "", fmt.Errorf("desteklenmeyen veritabanı türü: %q", typ)
	}
}

// DB collects health metrics for one database over a pooled connection.
type DB struct {
	name string
	typ  string
	db   *sql.DB
}

// NewDB opens a pooled handle for the configured database. It does not connect
// until first query, so a momentarily-down database never blocks startup.
func NewDB(cfg config.DatabaseConfig) (*DB, error) {
	drv, err := driverName(cfg.Type)
	if err != nil {
		return nil, err
	}
	db, err := sql.Open(drv, cfg.DSN)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(2)
	db.SetConnMaxLifetime(30 * time.Minute)
	return &DB{name: cfg.Name, typ: strings.ToLower(cfg.Type), db: db}, nil
}

// Close releases the pool.
func (d *DB) Close() error { return d.db.Close() }

// Collect runs the health queries with a bounded timeout. A failure of any
// single query is recorded but never aborts the rest.
func (d *DB) Collect(timeout time.Duration) model.DBMetrics {
	m := model.DBMetrics{Name: d.name, Type: d.typ}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	start := time.Now()
	if err := d.db.PingContext(ctx); err != nil {
		m.Up, m.Error = false, err.Error()
		return m
	}
	m.Up = true
	m.LatencyMS = round1(float64(time.Since(start).Microseconds()) / 1000)

	switch d.typ {
	case Oracle:
		d.collectOracle(ctx, &m)
	case Postgres:
		d.collectPostgres(ctx, &m)
	case MySQL, "mariadb":
		d.collectMySQL(ctx, &m)
	}
	if m.MaxSessions > 0 {
		m.SessionPercent = round1(100 * float64(m.Sessions) / float64(m.MaxSessions))
	}
	return m
}

func (d *DB) collectOracle(ctx context.Context, m *model.DBMetrics) {
	_ = d.db.QueryRowContext(ctx, "SELECT status FROM v$instance").Scan(&m.Status)
	var up sql.NullFloat64
	if d.db.QueryRowContext(ctx, "SELECT (SYSDATE - startup_time) * 86400 FROM v$instance").Scan(&up) == nil && up.Valid {
		m.UptimeSec = int64(up.Float64)
	}
	_ = d.db.QueryRowContext(ctx, "SELECT count(*) FROM v$session WHERE type = 'USER'").Scan(&m.Sessions)
	var mx sql.NullInt64
	if d.db.QueryRowContext(ctx, "SELECT to_number(value) FROM v$parameter WHERE name = 'sessions'").Scan(&mx) == nil && mx.Valid {
		m.MaxSessions = mx.Int64
	}
	_ = d.db.QueryRowContext(ctx, "SELECT count(*) FROM v$session WHERE blocking_session IS NOT NULL").Scan(&m.BlockingSessions)
	var hit sql.NullFloat64
	if d.db.QueryRowContext(ctx, `
SELECT round((1 - (phy.value / nullif(cur.value + con.value, 0))) * 100, 2)
FROM v$sysstat cur, v$sysstat con, v$sysstat phy
WHERE cur.name = 'db block gets' AND con.name = 'consistent gets' AND phy.name = 'physical reads'`).Scan(&hit) == nil && hit.Valid {
		m.BufferHitRatio = round1(hit.Float64)
	}
	if rows, err := d.db.QueryContext(ctx,
		"SELECT tablespace_name, round(used_percent, 1) FROM dba_tablespace_usage_metrics ORDER BY used_percent DESC"); err == nil {
		for rows.Next() {
			var s model.DBSpace
			if rows.Scan(&s.Name, &s.UsedPercent) == nil {
				m.Spaces = append(m.Spaces, s)
			}
		}
		rows.Close()
	}
}

func (d *DB) collectPostgres(ctx context.Context, m *model.DBMetrics) {
	m.Status = "OPEN"
	var up sql.NullFloat64
	if d.db.QueryRowContext(ctx, "SELECT extract(epoch FROM now() - pg_postmaster_start_time())").Scan(&up) == nil && up.Valid {
		m.UptimeSec = int64(up.Float64)
	}
	_ = d.db.QueryRowContext(ctx, "SELECT count(*) FROM pg_stat_activity").Scan(&m.Sessions)
	var mx sql.NullInt64
	if d.db.QueryRowContext(ctx, "SELECT setting::int FROM pg_settings WHERE name = 'max_connections'").Scan(&mx) == nil && mx.Valid {
		m.MaxSessions = mx.Int64
	}
	_ = d.db.QueryRowContext(ctx, "SELECT count(*) FROM pg_locks WHERE NOT granted").Scan(&m.BlockingSessions)
	var hit sql.NullFloat64
	if d.db.QueryRowContext(ctx,
		"SELECT round(100 * sum(blks_hit) / nullif(sum(blks_hit + blks_read), 0), 1) FROM pg_stat_database").Scan(&hit) == nil && hit.Valid {
		m.BufferHitRatio = round1(hit.Float64)
	}
}

func (d *DB) collectMySQL(ctx context.Context, m *model.DBMetrics) {
	m.Status = "OPEN"
	_ = d.db.QueryRowContext(ctx, "SELECT @@global.uptime").Scan(&m.UptimeSec)
	_ = d.db.QueryRowContext(ctx, "SELECT count(*) FROM information_schema.processlist").Scan(&m.Sessions)
	_ = d.db.QueryRowContext(ctx, "SELECT @@max_connections").Scan(&m.MaxSessions)
	_ = d.db.QueryRowContext(ctx,
		"SELECT count(*) FROM information_schema.processlist WHERE state LIKE '%lock%'").Scan(&m.BlockingSessions)

	var reqName, readName string
	var reqs, reads sql.NullFloat64
	_ = d.db.QueryRowContext(ctx, "SHOW GLOBAL STATUS LIKE 'Innodb_buffer_pool_read_requests'").Scan(&reqName, &reqs)
	_ = d.db.QueryRowContext(ctx, "SHOW GLOBAL STATUS LIKE 'Innodb_buffer_pool_reads'").Scan(&readName, &reads)
	if reqs.Valid && reqs.Float64 > 0 {
		m.BufferHitRatio = round1(100 * (1 - reads.Float64/reqs.Float64))
	}
}
