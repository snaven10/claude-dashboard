// Package server expone el frontend embebido, GET /api/sessions y el stream
// SSE GET /api/events. Hay UN solo poller (DD-4) y bind a loopback (DD-7).
package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"claude-dashboard/internal/agents"
	"claude-dashboard/internal/history"
	"claude-dashboard/internal/orq"
	"claude-dashboard/internal/transcript"
)

const (
	subBuffer        = 4
	defaultHeartbeat = 15 * time.Second
	shutdownGrace    = 5 * time.Second
)

// Server junta poller, broadcaster y rutas. Las dependencias externas se
// inyectan como función para testear sin ejecutar `claude`.
type Server struct {
	List     func(ctx context.Context) ([]agents.Session, error)
	Summary  func(cwd, sessionID string) (transcript.Detail, error)
	Usage    func(cwd, sessionID string) (*transcript.Tokens, error)
	Activity func(cwd, sessionID string, now time.Time) (*transcript.Context, *transcript.Rate, error)
	Timeline func(cwd, sessionID string) ([]transcript.Event, error)
	Errors   func(cwd, sessionID string, now time.Time) (*transcript.ErrorStats, error)
	// Extras: prs, artifacts, lines, permissionMode y subagents (PLAN-008, DD-35).
	Extras func(cwd, sessionID string, now time.Time) (*transcript.Extras, error)
	// ErrorsWarn: errors.high si count >= ErrorsWarn (PLAN-005 DD-22); 0 desactiva.
	ErrorsWarn int
	// Umbrales de PLAN-003; 0 desactiva el aviso correspondiente.
	ContextWarn int64 // context.warn si current >= ContextWarn
	RateOutput  int64 // rate.high si output > RateOutput
	RateCalls   int   // rate.high si calls > RateCalls
	Orq         func() (map[string]orq.Job, error)
	// History devuelve el último historial calculado (PLAN-006, DD-27); nunca escanea.
	History    func(now time.Time) history.Response
	StaleAfter time.Duration // 0 desactiva stale
	Now        func() time.Time
	Static     fs.FS
	Interval   time.Duration
	Heartbeat  time.Duration
	Logf       func(format string, args ...any)
	// RemoteAddr: segundo listener loopback para `tailscale serve` (PLAN-011,
	// DD-43). Vacío = un solo listener, como siempre.
	RemoteAddr string
	// PublicURL: origen https normalizado del modo remoto (DD-44); vacío = apagado.
	PublicURL string

	mu       sync.Mutex
	hash     [32]byte
	have     bool
	current  []byte
	sessions []agents.Session
	subs     map[chan []byte]struct{}

	// waitMu protege waitLog: sessionId → último motivo logueado de "espera sin ask".
	waitMu  sync.Mutex
	waitLog map[string]string
}

func (s *Server) logf(format string, args ...any) {
	if s.Logf != nil {
		s.Logf(format, args...)
		return
	}
	log.Printf(format, args...)
}

// subscribe registra un suscriptor con canal buffered y devuelve el snapshot
// actual (nil si todavía no hay).
func (s *Server) subscribe() (chan []byte, []byte) {
	ch := make(chan []byte, subBuffer)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.subs == nil {
		s.subs = map[chan []byte]struct{}{}
	}
	s.subs[ch] = struct{}{}
	return ch, s.current
}

func (s *Server) unsubscribe(ch chan []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.subs[ch]; ok {
		delete(s.subs, ch)
		close(ch)
	}
}

// broadcast nunca bloquea: el suscriptor con el buffer lleno se descarta.
func (s *Server) broadcast(payload []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for ch := range s.subs {
		select {
		case ch <- payload:
		default:
			delete(s.subs, ch)
			close(ch)
			s.logf("sse: suscriptor lento descartado")
		}
	}
}

// Handler devuelve las rutas.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/sessions", s.handleSessions)
	mux.HandleFunc("GET /api/events", s.handleEvents)
	mux.HandleFunc("GET /api/sessions/{sessionId}/timeline", s.handleTimeline)
	mux.HandleFunc("GET /api/history", s.handleHistory)
	if s.Static != nil {
		mux.Handle("/", staticHandler(s.Static))
	}
	return mux
}

// staticHandler sirve el frontend y fija las cabeceras de la PWA (DD-54):
// *.webmanifest como application/manifest+json y /sw.js sin caché, para que
// el navegador vea siempre la versión nueva del service worker.
func staticHandler(fsys fs.FS) http.Handler {
	files := http.FileServer(http.FS(fsys))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, ".webmanifest"):
			w.Header().Set("Content-Type", "application/manifest+json")
		case r.URL.Path == "/sw.js":
			w.Header().Set("Cache-Control", "no-cache")
		}
		files.ServeHTTP(w, r)
	})
}

type originKey struct{}

// isRemote dice si la petición entró por el listener remoto (DD-43). Se decide
// por el listener, no por RemoteAddr ni cabeceras: `tailscale serve` es un
// proxy local y todo llega desde 127.0.0.1.
func isRemote(r *http.Request) bool {
	v, _ := r.Context().Value(originKey{}).(bool)
	return v
}

// newHTTPServer arma un http.Server que marca el origen en el contexto de cada
// conexión; BaseContext es por http.Server, de ahí que haya uno por listener.
func (s *Server) newHTTPServer(remote bool) *http.Server {
	return &http.Server{
		Handler:           s.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		BaseContext: func(net.Listener) context.Context {
			return context.WithValue(context.Background(), originKey{}, remote)
		},
	}
}

func (s *Server) snapshot() []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.current
}

func (s *Server) handleSessions(w http.ResponseWriter, r *http.Request) {
	body := s.snapshot()
	if body == nil {
		if _, err := s.Refresh(r.Context()); err != nil {
			s.logf("api/sessions: %v", err)
			http.Error(w, "no se pudieron listar las sesiones: "+err.Error(), http.StatusServiceUnavailable)
			return
		}
		body = s.snapshot()
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(body)
}

// handleTimeline sirve los últimos eventos de una sesión. El id se valida
// contra el último snapshot: nunca se usa para armar una ruta sin validar.
func (s *Server) handleTimeline(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("sessionId")
	s.mu.Lock()
	have := s.have
	s.mu.Unlock()
	if !have {
		if _, err := s.Refresh(r.Context()); err != nil {
			s.logf("timeline: %v", err)
			http.Error(w, "no se pudieron listar las sesiones: "+err.Error(), http.StatusServiceUnavailable)
			return
		}
	}
	var found *agents.Session
	s.mu.Lock()
	for i := range s.sessions {
		if s.sessions[i].SessionID == id && id != "" {
			c := s.sessions[i]
			found = &c
			break
		}
	}
	s.mu.Unlock()
	if found == nil {
		http.NotFound(w, r)
		return
	}
	events := []transcript.Event{}
	if s.Timeline != nil {
		ev, err := s.Timeline(found.Cwd, found.SessionID)
		if err != nil {
			s.logf("timeline %s: %v", id, err)
		} else if ev != nil {
			events = ev
		}
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(struct {
		SessionID string             `json:"sessionId"`
		Events    []transcript.Event `json:"events"`
	}{id, events})
}

// handleHistory sirve el último historial calculado por el escáner aparte.
func (s *Server) handleHistory(w http.ResponseWriter, r *http.Request) {
	now := time.Now()
	if s.Now != nil {
		now = s.Now()
	}
	resp := history.Response{GeneratedAt: now.UnixMilli(), Days: []history.Day{}}
	if s.History != nil {
		resp = s.History(now)
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(resp)
}

func writeEvent(w http.ResponseWriter, f http.Flusher, payload []byte) error {
	if _, err := fmt.Fprintf(w, "event: snapshot\ndata: %s\n\n", payload); err != nil {
		return err
	}
	f.Flush()
	return nil
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	f, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming no soportado", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Connection", "keep-alive")

	ch, cur := s.subscribe()
	defer s.unsubscribe(ch)

	if cur == nil { // aún no hubo ciclo: intento uno para no dejar al cliente en blanco
		if _, err := s.Refresh(r.Context()); err != nil {
			s.logf("sse: %v", err)
		}
		cur = s.snapshot()
	}
	w.WriteHeader(http.StatusOK)
	if cur != nil {
		if err := writeEvent(w, f, cur); err != nil {
			return
		}
	} else {
		fmt.Fprint(w, ": connected\n\n")
		f.Flush()
	}

	hb := s.Heartbeat
	if hb <= 0 {
		hb = defaultHeartbeat
	}
	t := time.NewTicker(hb)
	defer t.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case payload, open := <-ch:
			if !open { // descartado por lento
				return
			}
			if err := writeEvent(w, f, payload); err != nil {
				return
			}
		case <-t.C:
			if _, err := fmt.Fprint(w, ": ping\n\n"); err != nil {
				return
			}
			f.Flush()
		}
	}
}

// ValidateAddr aplica DD-7: solo loopback salvo allowRemote.
func ValidateAddr(addr string, allowRemote bool) error {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("addr inválido %q: %w", addr, err)
	}
	if allowRemote {
		return nil
	}
	if host == "localhost" {
		return nil
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		return nil
	}
	return fmt.Errorf("addr %q no es loopback: el panel expone prompts; usá 127.0.0.1 o -allow-remote", addr)
}

// ListenAndServe arranca el poller y el HTTP; con ctx cancelado hace shutdown
// limpio. Si RemoteAddr != "" abre además el listener remoto (DD-43).
func (s *Server) ListenAndServe(ctx context.Context, addr string) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("escuchando en %s: %w", addr, err)
	}
	var rln net.Listener
	if s.RemoteAddr != "" {
		rln, err = net.Listen("tcp", s.RemoteAddr)
		if err != nil {
			_ = ln.Close()
			return fmt.Errorf("escuchando (remoto) en %s: %w", s.RemoteAddr, err)
		}
	}
	return s.Serve(ctx, ln, rln)
}

// Serve hace lo mismo que ListenAndServe sobre listeners ya abiertos (remote
// puede ser nil). Hay UN poller aunque haya dos listeners.
func (s *Server) Serve(ctx context.Context, ln, remote net.Listener) error {
	pctx, stopPoll := context.WithCancel(ctx)
	defer stopPoll()
	pollDone := make(chan struct{})
	go func() { defer close(pollDone); s.Poll(pctx) }()

	srvs := []*http.Server{s.newHTTPServer(false)}
	lns := []net.Listener{ln}
	if remote != nil {
		srvs = append(srvs, s.newHTTPServer(true))
		lns = append(lns, remote)
	}
	errc := make(chan error, len(srvs))
	for i := range srvs {
		go func(srv *http.Server, l net.Listener) { errc <- srv.Serve(l) }(srvs[i], lns[i])
	}
	s.logf("claude-dashboard escuchando en http://%s", ln.Addr())
	if remote != nil {
		s.logf("listener remoto (para tailscale serve) en http://%s", remote.Addr())
	}

	select {
	case err := <-errc:
		stopPoll()
		<-pollDone
		for _, srv := range srvs {
			_ = srv.Close()
		}
		return fmt.Errorf("sirviendo: %w", err)
	case <-ctx.Done():
	}
	stopPoll()
	<-pollDone
	// SSE son conexiones largas: cerramos suscriptores para que Shutdown no espere.
	s.mu.Lock()
	for ch := range s.subs {
		delete(s.subs, ch)
		close(ch)
	}
	s.mu.Unlock()
	sctx, cancel := context.WithTimeout(context.Background(), shutdownGrace)
	defer cancel()
	var wg sync.WaitGroup
	errs := make([]error, len(srvs))
	for i, srv := range srvs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := srv.Shutdown(sctx); err != nil && !errors.Is(err, http.ErrServerClosed) {
				_ = srv.Close()
				errs[i] = fmt.Errorf("shutdown: %w", err)
			}
		}()
	}
	wg.Wait()
	return errors.Join(errs...)
}
