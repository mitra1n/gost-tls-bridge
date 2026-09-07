// Package webui serves a small local control panel for the bridge on
// 127.0.0.1. It is pure net/http + embed, so it cross-compiles to every OS
// with no cgo. The panel talks to the same bridge/config code as the CLI.
package webui

import (
	"embed"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/mitra1n/gost-tls-bridge/internal/bridge"
	"github.com/mitra1n/gost-tls-bridge/internal/cert"
	"github.com/mitra1n/gost-tls-bridge/internal/config"
)

//go:embed index.html
var assets embed.FS

// Server holds the runtime state for the control panel.
type Server struct {
	cfgPath string
}

// New builds a control-panel server bound to a config path.
func New(cfgPath string) *Server { return &Server{cfgPath: cfgPath} }

// Handler returns the HTTP mux for the panel.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/api/status", s.handleStatus)
	mux.HandleFunc("/api/start", s.action(func(c *config.Config) error { _, err := bridge.Start(c); return err }))
	mux.HandleFunc("/api/stop", s.action(func(c *config.Config) error { return bridge.Stop(c) }))
	mux.HandleFunc("/api/restart", s.action(func(c *config.Config) error {
		_ = bridge.Stop(c)
		time.Sleep(300 * time.Millisecond)
		_, err := bridge.Start(c)
		return err
	}))
	mux.HandleFunc("/api/certgen", s.action(func(c *config.Config) error {
		return cert.GenerateRelayPEM(c.RelayCert, c.TargetHost)
	}))
	mux.HandleFunc("/api/check", s.handleCheck)
	mux.HandleFunc("/api/burp", s.handleBurp)
	mux.HandleFunc("/api/config", s.handleConfig)
	return mux
}

func (s *Server) load() (*config.Config, error) {
	c, err := config.Load(s.cfgPath)
	if err != nil {
		return nil, err
	}
	return c, c.Validate()
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, err error) {
	writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": err.Error()})
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	b, err := assets.ReadFile("index.html")
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(b)
}

type statusResp struct {
	OK         bool   `json:"ok"`
	Error      string `json:"error,omitempty"`
	Target     string `json:"target"`
	Backend    string `json:"backend"`
	FrontProc  bool   `json:"front_proc"`
	FrontPort  bool   `json:"front_port"`
	FrontAddr  string `json:"front_addr"`
	GostProc   bool   `json:"gost_proc"`
	GostPort   bool   `json:"gost_port"`
	GostAddr   string `json:"gost_addr"`
	RelayFound bool   `json:"relay_found"`
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	c, err := s.load()
	if err != nil {
		writeErr(w, err)
		return
	}
	st := bridge.Probe(c)
	_, relayErr := os.Stat(c.RelayCert)
	writeJSON(w, http.StatusOK, statusResp{
		OK:         true,
		Target:     fmt.Sprintf("%s:%d", c.TargetHost, c.TargetPort),
		Backend:    string(c.ResolveBackend()),
		FrontProc:  st.FrontUp,
		FrontPort:  st.FrontPortOpen,
		FrontAddr:  c.FrontListen,
		GostProc:   st.GostUp,
		GostPort:   st.GostPortOpen,
		GostAddr:   c.GostListen,
		RelayFound: relayErr == nil,
	})
}

// action wraps a bridge mutation as a POST endpoint returning {ok,error}.
func (s *Server) action(fn func(*config.Config) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"ok": false, "error": "POST required"})
			return
		}
		c, err := s.load()
		if err != nil {
			writeErr(w, err)
			return
		}
		if err := fn(c); err != nil {
			writeErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	}
}

func (s *Server) handleCheck(w http.ResponseWriter, r *http.Request) {
	c, err := s.load()
	if err != nil {
		writeErr(w, err)
		return
	}
	st := bridge.Probe(c)
	if !st.FrontPortOpen {
		writeErr(w, fmt.Errorf("front-порт %s закрыт — мост не запущен?", c.FrontListen))
		return
	}
	status, err := bridge.CurlThroughBridge(c)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "status": status})
}

func (s *Server) handleBurp(w http.ResponseWriter, r *http.Request) {
	c, err := s.load()
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "text": bridge.BurpHints(c)})
}

func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	b, err := os.ReadFile(s.cfgPath)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "path": s.cfgPath, "text": string(b)})
}
