// Command srvmon monitors a Windows server and its databases (Oracle, PostgreSQL,
// MySQL/MariaDB), inspects Windows services and the Event Log, serves a live
// diagnostic web dashboard with an in-browser settings page, and raises
// e-mail / SMS alarms. It runs as a single self-contained executable.
package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"time"

	"srvmon/internal/alert"
	"srvmon/internal/assess"
	"srvmon/internal/collect"
	"srvmon/internal/config"
	"srvmon/internal/digest"
	"srvmon/internal/forecast"
	"srvmon/internal/model"
	"srvmon/internal/notify"
	"srvmon/internal/store"
	"srvmon/internal/web"
)

const forecastHorizon = 48 * time.Hour

// deepState holds the latest slow-scan results (services, events, watched items).
type deepState struct {
	mu       sync.RWMutex
	services []model.ServiceStatus
	events   []model.EventEntry
	watch    []model.WatchStatus
}

func (d *deepState) set(s []model.ServiceStatus, e []model.EventEntry, w []model.WatchStatus) {
	d.mu.Lock()
	d.services, d.events, d.watch = s, e, w
	d.mu.Unlock()
}

func (d *deepState) get() ([]model.ServiceStatus, []model.EventEntry, []model.WatchStatus) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.services, d.events, d.watch
}

// core is one running monitoring pipeline built from a configuration. It can be
// stopped and replaced wholesale when the configuration changes.
type core struct {
	cfg    *config.Config
	demo   bool
	ring   *store.Ring
	engine *alert.Engine
	byRule map[string][]string
	router *notify.Router
	sys    *collect.System
	dbs    []*collect.DB
	deep   *deepState
	cancel context.CancelFunc
}

func newCore(cfg *config.Config, demo bool) (*core, error) {
	rules, err := cfg.EngineRules()
	if err != nil {
		return nil, err
	}
	var channels []notify.Channel
	if e := notify.NewEmail(cfg.Email); e != nil {
		channels = append(channels, e)
	}
	if sm := notify.NewSMS(cfg.SMS); sm != nil {
		channels = append(channels, sm)
	}
	capacity := cfg.HistorySamples
	if capacity <= 0 {
		capacity = 240
	}
	c := &core{
		cfg: cfg, demo: demo,
		ring:   store.New(capacity),
		engine: alert.New(rules),
		byRule: cfg.ChannelsByRule(),
		router: notify.NewRouter(channels...),
		sys:    collect.NewSystem(),
		deep:   &deepState{},
	}
	for _, dc := range cfg.Databases {
		db, err := collect.NewDB(dc)
		if err != nil {
			log.Printf("veritabanı %q başlatılamadı: %v", dc.Name, err)
			continue
		}
		c.dbs = append(c.dbs, db)
	}
	return c, nil
}

func (c *core) watchAll() []model.WatchStatus {
	var w []model.WatchStatus
	w = append(w, collect.CollectWatchedServices(c.cfg.Watch.Services)...)
	w = append(w, collect.CollectWatchedProcesses(c.cfg.Watch.Processes)...)
	w = append(w, collect.CollectWatchedEndpoints(c.cfg.Watch.Endpoints, 5*time.Second)...)
	return w
}

func (c *core) scanDeep() {
	if c.demo {
		c.deep.set(collect.DemoServices(), collect.DemoEvents(time.Now()), collect.DemoWatch())
		return
	}
	c.deep.set(collect.CollectServices(),
		collect.CollectEvents(c.cfg.EventWindow(), c.cfg.EventLimit()), c.watchAll())
}

func (c *core) poll() {
	sample := model.Sample{Time: time.Now(), Server: c.cfg.ServerName}
	sample.System = c.sys.Collect()
	timeout := clampTimeout(c.cfg.PollInterval() - time.Second)
	for _, db := range c.dbs {
		sample.Databases = append(sample.Databases, db.Collect(timeout))
	}
	if c.demo && len(sample.Databases) == 0 {
		sample.Databases = collect.DemoDatabases()
	}
	sample.Services, sample.Events, sample.Watch = c.deep.get()
	sample.Values = collect.Flatten(sample)
	c.ring.Add(sample)

	preds := forecast.All(c.ring.History(), forecastHorizon)
	sample.Values["forecast.hours_to_full"] = forecast.MinETAHours(preds)

	for _, ev := range c.engine.Evaluate(sample.Values, sample.Time) {
		c.router.Dispatch(c.byRule[ev.RuleID], notify.FromEvent(c.cfg.ServerName, ev))
		log.Printf("ALARM [%s] %s = %.1f (eşik %.1f)", ev.Level, ev.Key, ev.Value, ev.Threshold)
	}
}

func (c *core) sendDigest(now time.Time) {
	sample, ok := c.ring.Latest()
	if !ok {
		return
	}
	preds := forecast.All(c.ring.History(), forecastHorizon)
	health := assess.Merge(assess.Evaluate(sample, c.engine.Active()), preds)
	subject, body := digest.Build(sample, health, preds, now)
	to := c.cfg.Digest.Recipients(c.cfg.Email.To)
	if err := notify.SendHTML(c.cfg.Email, to, subject, body); err != nil {
		log.Printf("durum raporu gönderilemedi: %v", err)
	} else {
		log.Printf("durum raporu e-postası gönderildi (%d alıcı)", len(to))
	}
}

// start launches the collection, alarm and digest loops under a child context.
func (c *core) start(parent context.Context) {
	ctx, cancel := context.WithCancel(parent)
	c.cancel = cancel
	c.scanDeep()
	c.poll()

	go func() {
		t := time.NewTicker(c.cfg.PollInterval())
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				c.poll()
			}
		}
	}()
	go func() {
		t := time.NewTicker(c.cfg.DeepInterval())
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				c.scanDeep()
			}
		}
	}()
	if c.cfg.Digest.Enabled {
		go func() {
			for {
				next, err := c.cfg.Digest.NextDigest(time.Now())
				if err != nil {
					log.Printf("digest zamanlaması geçersiz: %v", err)
					return
				}
				timer := time.NewTimer(time.Until(next))
				select {
				case <-ctx.Done():
					timer.Stop()
					return
				case <-timer.C:
					c.sendDigest(time.Now())
				}
			}
		}()
	}
}

func (c *core) stop() {
	if c.cancel != nil {
		c.cancel()
	}
	for _, db := range c.dbs {
		_ = db.Close()
	}
}

// supervisor holds the current core and swaps it on config reload. It
// implements web.Provider so the HTTP server survives reloads untouched.
type supervisor struct {
	mu         sync.RWMutex
	cur        *core
	root       context.Context
	demo       bool
	configPath string
}

func (s *supervisor) current() *core {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cur
}

func (s *supervisor) ServerName() string { return s.current().cfg.ServerName }
func (s *supervisor) Token() string      { return s.current().cfg.Web.Token }
func (s *supervisor) Latest() (model.Sample, bool) {
	return s.current().ring.Latest()
}
func (s *supervisor) History() []model.Sample        { return s.current().ring.History() }
func (s *supervisor) Active() map[string]alert.Level { return s.current().engine.Active() }

// reload validates new config bytes, persists them, and swaps the core.
// The web listen address and token change on disk but take effect only on the
// next process start, because the HTTP server keeps running here.
func (s *supervisor) reload(b []byte) error {
	cfg, err := config.Parse(b)
	if err != nil {
		return err
	}
	if err := cfg.Save(s.configPath); err != nil {
		return err
	}
	nc, err := newCore(cfg, s.demo)
	if err != nil {
		return err
	}
	nc.start(s.root)

	s.mu.Lock()
	old := s.cur
	s.cur = nc
	s.mu.Unlock()
	old.stop()
	log.Printf("yapılandırma yeniden yüklendi (%d veritabanı, %d kural)", len(cfg.Databases), len(cfg.Rules))
	return nil
}

func main() {
	cfgPath := flag.String("config", "config.json", "yapılandırma dosyası yolu")
	demo := flag.Bool("demo", false, "örnek servis/olay/veritabanı verisiyle önizleme")
	digestNow := flag.Bool("digest-now", false, "başlangıçta bir durum raporu e-postası gönder ve test et")
	digestFile := flag.String("digest-file", "", "durum raporunun HTML'ini bu dosyaya yazıp çık (önizleme)")
	flag.Parse()

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		log.Fatalf("yapılandırma yüklenemedi: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	first, err := newCore(cfg, *demo)
	if err != nil {
		log.Fatalf("başlatılamadı: %v", err)
	}
	sup := &supervisor{cur: first, root: ctx, demo: *demo, configPath: *cfgPath}
	first.start(ctx)

	if *digestFile != "" {
		sample, _ := first.ring.Latest()
		preds := forecast.All(first.ring.History(), forecastHorizon)
		health := assess.Merge(assess.Evaluate(sample, first.engine.Active()), preds)
		_, body := digest.Build(sample, health, preds, time.Now())
		if err := os.WriteFile(*digestFile, []byte(body), 0o644); err != nil {
			log.Fatalf("digest yazılamadı: %v", err)
		}
		log.Printf("durum raporu HTML'i yazıldı: %s", *digestFile)
		return
	}
	if *digestNow {
		if cfg.Email.Host == "" {
			log.Println("-digest-now: email.host boş, rapor gönderilemedi")
		} else {
			first.sendDigest(time.Now())
		}
	}

	srv := web.New(sup, *cfgPath, sup.reload).Handler()
	httpSrv := &http.Server{Addr: cfg.Web.Listen, Handler: srv, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		shown := cfg.Web.Listen
		if len(shown) > 0 && shown[0] == ':' {
			shown = "localhost" + shown
		}
		log.Printf("pano hazır: http://%s  ·  ayarlar: http://%s/ayarlar", shown, shown)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("web sunucusu: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("kapatılıyor...")
	shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	_ = httpSrv.Shutdown(shutCtx)
	cancel()
	sup.current().stop()
}

func clampTimeout(d time.Duration) time.Duration {
	if d > 10*time.Second {
		return 10 * time.Second
	}
	if d < 3*time.Second {
		return 3 * time.Second
	}
	return d
}
