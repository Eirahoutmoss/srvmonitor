// Package web serves the live dashboard, its JSON API, and a settings page
// that reads and writes the configuration file.
package web

import (
	"embed"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"time"

	"srvmon/internal/alert"
	"srvmon/internal/assess"
	"srvmon/internal/forecast"
	"srvmon/internal/model"
)

//go:embed assets/index.html assets/settings.html
var assets embed.FS

const forecastHorizon = 48 * time.Hour

// Provider exposes the live monitoring state to the web layer. The concrete
// implementation (the supervisor) can swap its backing core on config reload
// without the HTTP server restarting.
type Provider interface {
	ServerName() string
	Token() string
	Latest() (model.Sample, bool)
	History() []model.Sample
	Active() map[string]alert.Level
}

// Server wires the provider and config file to HTTP handlers.
type Server struct {
	prov       Provider
	configPath string
	reload     func([]byte) error
}

// New builds a dashboard server. reload validates+persists+applies new config
// bytes; it returns a user-facing error on failure.
func New(prov Provider, configPath string, reload func([]byte) error) *Server {
	return &Server{prov: prov, configPath: configPath, reload: reload}
}

// Handler returns the HTTP routes.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/status", s.gate(s.status))
	mux.HandleFunc("/api/history", s.gate(s.history))
	mux.HandleFunc("/api/config", s.gate(s.config))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("ok")) })
	mux.HandleFunc("/ayarlar", s.page("assets/settings.html"))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		s.page("assets/index.html")(w, r)
	})
	return mux
}

func (s *Server) page(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		b, _ := assets.ReadFile(name)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(b)
	}
}

// gate optionally enforces the shared token on API endpoints.
func (s *Server) gate(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if tok := s.prov.Token(); tok != "" && r.URL.Query().Get("token") != tok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		h(w, r)
	}
}

type statusResp struct {
	Server      string                `json:"server"`
	Generated   time.Time             `json:"generated"`
	HasData     bool                  `json:"has_data"`
	Health      assess.Health         `json:"health"`
	Predictions []forecast.Prediction `json:"predictions"`
	Sample      model.Sample          `json:"sample"`
	Active      map[string]string     `json:"active"`
}

func (s *Server) status(w http.ResponseWriter, _ *http.Request) {
	sample, ok := s.prov.Latest()
	activeLvl := s.prov.Active()
	active := map[string]string{}
	for k, lvl := range activeLvl {
		active[k] = lvl.String()
	}
	preds := forecast.All(s.prov.History(), forecastHorizon)
	health := assess.Merge(assess.Evaluate(sample, activeLvl), preds)
	writeJSON(w, statusResp{
		Server: s.prov.ServerName(), Generated: time.Now(), HasData: ok,
		Health: health, Predictions: preds, Sample: sample, Active: active,
	})
}

func (s *Server) history(w http.ResponseWriter, _ *http.Request) {
	hist := s.prov.History()
	type point struct {
		T    time.Time `json:"t"`
		CPU  float64   `json:"cpu"`
		Mem  float64   `json:"mem"`
		Swap float64   `json:"swap"`
	}
	pts := make([]point, 0, len(hist))
	for _, h := range hist {
		pts = append(pts, point{T: h.Time, CPU: h.System.CPUPercent,
			Mem: h.System.MemPercent, Swap: h.System.SwapPercent})
	}
	writeJSON(w, pts)
}

// config serves the current config file (GET) or applies a new one (POST).
func (s *Server) config(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		b, err := os.ReadFile(s.configPath)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Write(b)
	case http.MethodPost:
		b, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
		if err != nil {
			writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
			return
		}
		if err := s.reload(b); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
			return
		}
		writeJSON(w, map[string]any{"ok": true})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(v)
}
