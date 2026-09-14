package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"donchian.trade/bot/internal/engine"
	"donchian.trade/bot/internal/store"
)

type Server struct {
	Addr     string
	Password string
	Secure   bool
	Engine   *engine.Engine
	Store    *store.Store

	mu       sync.Mutex
	sessions map[string]time.Time
	fails    map[string][]time.Time
}

func New(addr, password string, secure bool, eng *engine.Engine, st *store.Store) *Server {
	return &Server{
		Addr: addr, Password: password, Secure: secure,
		Engine: eng, Store: st, sessions: map[string]time.Time{}, fails: map[string][]time.Time{},
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", s.health)
	mux.HandleFunc("POST /api/login", s.login)
	mux.HandleFunc("POST /api/logout", s.logout)
	mux.HandleFunc("GET /api/status", s.auth(s.status))
	mux.HandleFunc("GET /api/positions", s.auth(s.status))
	mux.HandleFunc("GET /api/trades", s.auth(s.trades))
	mux.HandleFunc("GET /api/equity", s.auth(s.equity))
	mux.HandleFunc("GET /api/stats", s.auth(s.stats))
	mux.HandleFunc("GET /api/events", s.auth(s.events))
	mux.HandleFunc("GET /api/telemetry", s.auth(s.telemetry))
	mux.HandleFunc("GET /api/mismatches", s.auth(s.mismatches))
	mux.HandleFunc("GET /api/report", s.auth(s.report))
	mux.HandleFunc("GET /api/cash", s.auth(s.cash))
	mux.HandleFunc("POST /api/kill", s.auth(s.kill))
	mux.HandleFunc("POST /api/resume", s.auth(s.resume))
	return withLog(mux)
}

func (s *Server) ListenAndServe() error {
	go s.gc()
	srv := &http.Server{Addr: s.Addr, Handler: s.Handler(), ReadHeaderTimeout: 10 * time.Second}
	return srv.ListenAndServe()
}

func (s *Server) gc() {
	t := time.NewTicker(30 * time.Minute)
	defer t.Stop()
	for range t.C {
		s.mu.Lock()
		now := time.Now()
		for k, exp := range s.sessions {
			if now.After(exp) {
				delete(s.sessions, k)
			}
		}
		s.mu.Unlock()
	}
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	ready, detail := s.Engine.Health()
	ws, wsErr := s.Engine.WSState()
	writeJSON(w, http.StatusOK, map[string]any{
		"ok": true, "ready": ready, "detail": detail, "ws": ws, "ws_error": wsErr,
	})
}

func clientIP(r *http.Request) string {
	if x := r.Header.Get("X-Forwarded-For"); x != "" {
		if i := strings.IndexByte(x, ','); i >= 0 {
			return strings.TrimSpace(x[:i])
		}
		return strings.TrimSpace(x)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func (s *Server) loginLocked(ip string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	cut := time.Now().Add(-10 * time.Minute)
	var keep []time.Time
	for _, t := range s.fails[ip] {
		if t.After(cut) {
			keep = append(keep, t)
		}
	}
	s.fails[ip] = keep
	return len(keep) >= 8
}

func (s *Server) noteLoginFail(ip string) {
	s.mu.Lock()
	s.fails[ip] = append(s.fails[ip], time.Now())
	s.mu.Unlock()
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	ip := clientIP(r)
	if s.loginLocked(ip) {
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "too many attempts"})
		return
	}
	var body struct {
		Password string `json:"password"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if body.Password == "" {
		body.Password = r.FormValue("password")
	}
	if body.Password != s.Password {
		s.noteLoginFail(ip)
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "bad password"})
		return
	}
	tok := randomToken()
	s.mu.Lock()
	s.sessions[tok] = time.Now().Add(14 * 24 * time.Hour)
	s.mu.Unlock()
	http.SetCookie(w, &http.Cookie{
		Name:     "donchian_session",
		Value:    tok,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   s.Secure,
		MaxAge:   14 * 24 * 3600,
	})
	writeJSON(w, http.StatusOK, map[string]string{"ok": "1"})
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie("donchian_session"); err == nil {
		s.mu.Lock()
		delete(s.sessions, c.Value)
		s.mu.Unlock()
	}
	http.SetCookie(w, &http.Cookie{Name: "donchian_session", Value: "", Path: "/", MaxAge: -1})
	writeJSON(w, http.StatusOK, map[string]string{"ok": "1"})
}

func (s *Server) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie("donchian_session")
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "auth"})
			return
		}
		s.mu.Lock()
		exp, ok := s.sessions[c.Value]
		s.mu.Unlock()
		if !ok || time.Now().After(exp) {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "auth"})
			return
		}
		next(w, r)
	}
}

func (s *Server) status(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.Engine.Snapshot(r.Context()))
}

func (s *Server) trades(w http.ResponseWriter, r *http.Request) {
	rows, err := s.Store.ListTrades(r.Context(), 300)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	out := make([]map[string]any, 0, len(rows))
	for _, t := range rows {
		out = append(out, map[string]any{
			"id": t.ID, "symbol": t.Symbol, "profile": t.Profile, "direction": t.Direction,
			"signal_time": t.SignalTime, "entry_time": t.EntryTime.Int64, "entry_price": t.EntryPrice.Float64,
			"stop": t.Stop.Float64, "exit_time": t.ExitTime.Int64, "exit_price": t.ExitPrice.Float64,
			"outcome": t.Outcome.String, "risk_distance": t.RiskDistance.Float64, "quantity": t.Quantity.Float64,
			"gross": t.Gross.Float64, "net": t.Net.Float64, "funding": t.Funding.Float64,
			"consec_losses":   t.ConsecLosses.Int64,
			"entry_px_shadow": t.EntryPxShadow, "entry_px_live": t.EntryPxLive,
			"exit_px_shadow": t.ExitPxShadow, "exit_px_live": t.ExitPxLive,
		})
	}
	writeJSON(w, 200, out)
}

func (s *Server) equity(w http.ResponseWriter, r *http.Request) {
	rows, err := s.Store.ListCurve(r.Context(), 4000)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	if len(rows) == 0 {
		writeJSON(w, 200, []store.CurvePoint{})
		return
	}
	writeJSON(w, 200, rows)
}

func (s *Server) telemetry(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, s.Engine.ComputeReport(r.Context()))
}

func (s *Server) mismatches(w http.ResponseWriter, r *http.Request) {
	rows, err := s.Store.ListMismatches(r.Context(), 0, 100)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, rows)
}

func (s *Server) cash(w http.ResponseWriter, r *http.Request) {
	rows, err := s.Store.ListCashFlows(r.Context(), 100)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	base, _ := s.Store.SumCash(r.Context())
	writeJSON(w, 200, map[string]any{"cash_base": base, "flows": rows})
}

func (s *Server) report(w http.ResponseWriter, r *http.Request) {
	date, body, verdict, err := s.Store.LatestDailyReport(r.Context())
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]any{"date_utc": date, "body": body, "verdict": verdict})
}

func (s *Server) events(w http.ResponseWriter, r *http.Request) {
	rows, err := s.Store.ListEvents(r.Context(), 100)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, rows)
}

func (s *Server) stats(w http.ResponseWriter, r *http.Request) {
	rows, err := s.Store.ClosedLiveTrades(r.Context())
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	var wins, losses int
	var gp, gl, net float64
	peak, eq := 0.0, 0.0
	maxDD := 0.0
	for _, t := range rows {
		n := t.Net.Float64
		net += n
		eq += n
		if eq > peak {
			peak = eq
		}
		if peak > 0 {
			dd := (eq - peak) / peak
			if dd < maxDD {
				maxDD = dd
			}
		}
		if n > 0 || t.Gross.Float64 > 0 {
			wins++
			gp += t.Gross.Float64
		} else {
			losses++
			gl += -t.Gross.Float64
		}
	}
	wr := 0.0
	if wins+losses > 0 {
		wr = float64(wins) / float64(wins+losses)
	}
	pf := 0.0
	if gl > 0 {
		pf = gp / gl
	}
	writeJSON(w, 200, map[string]any{
		"trades":        wins + losses,
		"wins":          wins,
		"losses":        losses,
		"win_rate":      wr,
		"profit_factor": pf,
		"net":           net,
		"max_dd":        maxDD,
	})
}

func (s *Server) kill(w http.ResponseWriter, r *http.Request) {
	if err := s.Engine.KillAll(r.Context()); err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]string{"ok": "1"})
}

func (s *Server) resume(w http.ResponseWriter, r *http.Request) {
	s.Engine.Resume(r.Context())
	writeJSON(w, 200, map[string]string{"ok": "1"})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func randomToken() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

func withLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
	})
}

func Ready(_ context.Context) {}
