package server

// Notificador de avisos push (PLAN-011, DD-52 y DD-53) y endpoints /api/push/*.

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"claude-dashboard/internal/agents"
	"claude-dashboard/internal/remote"
	"claude-dashboard/internal/webpush"
)

// Modos de -push-content (DD-53).
const (
	PushContentFull  = "full"
	PushContentTitle = "title"
	PushContentNone  = "none"
)

// ValidPushContent dice si v es un valor aceptado de -push-content.
func ValidPushContent(v string) bool {
	return v == PushContentFull || v == PushContentTitle || v == PushContentNone
}

const (
	// debounceCycles: ciclos seguidos en waiting antes de avisar (DD-52).
	debounceCycles = 2
	// groupAbove: con más candidatas que esto en un ciclo sale un solo aviso.
	groupAbove = 3
	// maxPushRunes: tope de runas de título/cuerpo (DD-53).
	maxPushRunes = 180
	// maxPushBytes: tope del JSON del payload (el cifrado admite 3993).
	maxPushBytes = 3000
	// abandonedAfter: DD-30, igual que isAbandoned en web/app.js.
	abandonedAfter = 24 * time.Hour
	// notifyQueue: avisos pendientes entre el poller y el envío.
	notifyQueue = 64

	pushTitleSuffix = " — esperándote"
	pushNoAttention = "necesita tu atención"
	localDeviceID   = "local"
)

var pushURLIDRe = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)

type notifyState struct {
	streak int       // ciclos seguidos en waiting
	armed  bool      // el ciclo previo a la racha no era waiting (hay transición real)
	last   time.Time // último aviso de esta sesión
}

type pushItem struct {
	payload []byte
	topic   string
}

// Notifier decide cuándo una sesión pasó a "esperándote" y manda el aviso a
// Enqueue. Observe nunca bloquea al poller: la entrega a Enqueue va por una
// cola propia con un worker.
type Notifier struct {
	// Now es el reloj; nil = time.Now.
	Now func() time.Time
	// Cooldown: no volver a avisar la misma sesión antes de este tiempo; 0 lo desactiva.
	Cooldown time.Duration
	// Content: "full" | "title" | "none"; cualquier otro valor se trata como "none".
	Content string
	// Enqueue recibe el payload ya armado (JSON) y el Topic. Lo llama un único
	// worker; puede tardar sin frenar al poller.
	Enqueue func(payload []byte, topic string)
	// Logf; nil = descartar.
	Logf func(format string, args ...any)

	mu      sync.Mutex
	started bool // ya hubo un primer ciclo
	state   map[string]*notifyState

	once    sync.Once
	out     chan pushItem
	pending sync.WaitGroup
}

func (n *Notifier) now() time.Time {
	if n.Now != nil {
		return n.Now()
	}
	return time.Now()
}

func (n *Notifier) logf(format string, args ...any) {
	if n.Logf != nil {
		n.Logf(format, args...)
	}
}

func pushWaiting(s *agents.Session) bool {
	return s.Status == agents.StatusWaiting || s.Status == agents.StatusBlocked
}

func pushAbandoned(s *agents.Session, now time.Time) bool {
	t := s.LastActivityAt
	if t == 0 {
		t = s.StartedAt
	}
	return t > 0 && now.UnixMilli()-t > abandonedAfter.Milliseconds()
}

// sessionKey replica rowKey del front: sessionId (o id) y '#pid'.
func sessionKey(s *agents.Session) string {
	k := s.SessionID
	if k == "" {
		k = s.ID
	}
	if s.PID > 0 {
		k += fmt.Sprintf("#%d", s.PID)
	}
	return k
}

// Observe se llama en CADA ciclo del poller con las sesiones actuales.
func (n *Notifier) Observe(sessions []agents.Session) {
	now := n.now()
	var fire []agents.Session

	n.mu.Lock()
	first := !n.started
	n.started = true
	next := make(map[string]*notifyState, len(sessions))
	for i := range sessions {
		s := &sessions[i]
		key := sessionKey(s)
		st := n.state[key]
		if st == nil {
			// En el primer ciclo solo se registra: una sesión que ya esperaba al
			// arrancar no es una transición. Después, toda sesión nueva sí lo es.
			st = &notifyState{armed: !first}
		}
		next[key] = st
		if !pushWaiting(s) || pushAbandoned(s, now) {
			st.streak = 0
			st.armed = true
			continue
		}
		st.streak++
		if st.streak != debounceCycles || !st.armed {
			continue
		}
		if n.Cooldown > 0 && !st.last.IsZero() && now.Sub(st.last) < n.Cooldown {
			continue
		}
		st.last = now
		fire = append(fire, *s)
	}
	n.state = next // las ausentes se olvidan
	n.mu.Unlock()

	if len(fire) == 0 {
		return
	}
	if len(fire) > groupAbove {
		n.push(n.groupPayload(fire), webpush.TopicFor("claude-dashboard:group"))
		return
	}
	for i := range fire {
		n.push(n.sessionPayload(&fire[i]), webpush.TopicFor(fire[i].SessionID))
	}
}

// push encola sin bloquear; si la cola propia está llena, descarta y loguea.
func (n *Notifier) push(payload []byte, topic string) {
	if payload == nil || n.Enqueue == nil {
		return
	}
	n.once.Do(func() {
		n.out = make(chan pushItem, notifyQueue)
		go func() {
			for it := range n.out {
				n.Enqueue(it.payload, it.topic)
				n.pending.Done()
			}
		}()
	})
	n.pending.Add(1)
	select {
	case n.out <- pushItem{payload, topic}:
	default:
		n.pending.Done()
		n.logf("push: cola del notificador llena (%d); aviso descartado", notifyQueue)
	}
}

// Flush espera a que los avisos ya encolados hayan pasado por Enqueue.
func (n *Notifier) Flush() { n.pending.Wait() }

// Len devuelve cuántas sesiones recuerda (para verificar que no crece).
func (n *Notifier) Len() int {
	n.mu.Lock()
	defer n.mu.Unlock()
	return len(n.state)
}

type pushPayload struct {
	V     int    `json:"v"`
	Title string `json:"title"`
	Body  string `json:"body"`
	Tag   string `json:"tag"`
	URL   string `json:"url"`
}

// oneLine quita controles y colapsa espacios (los prompts traen saltos de línea).
func oneLine(s string) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, s)
	return strings.Join(strings.Fields(s), " ")
}

// truncRunes recorta a max runas; si recorta termina en "…" (dentro del tope).
func truncRunes(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	if max <= 1 {
		return "…"
	}
	return string(r[:max-1]) + "…"
}

func (n *Notifier) mode() string {
	if ValidPushContent(n.Content) {
		return n.Content
	}
	return PushContentNone
}

func (n *Notifier) sessionPayload(s *agents.Session) []byte {
	var p pushPayload
	p.V = 1
	switch n.mode() {
	case PushContentNone:
		p.Title, p.Body, p.Tag, p.URL = "claude-dashboard", "Una sesión te espera", "claude-dashboard", "/"
	default:
		name := oneLine(s.Title)
		if name == "" {
			name = oneLine(s.Name)
		}
		if name == "" {
			name = oneLine(s.ID)
		}
		p.Title = truncRunes(name, maxPushRunes-len([]rune(pushTitleSuffix))) + pushTitleSuffix
		project := oneLine(s.Project)
		if n.mode() == PushContentFull {
			text := ""
			if s.Ask != nil {
				text = oneLine(s.Ask.Text)
			}
			if text == "" && s.WaitingFor != nil {
				text = oneLine(*s.WaitingFor)
			}
			if text == "" {
				text = pushNoAttention
			}
			if project != "" {
				text = project + ": " + text
			}
			p.Body = truncRunes(text, maxPushRunes)
		} else {
			p.Body = truncRunes(project, maxPushRunes)
			if p.Body == "" {
				p.Body = "Una sesión te espera"
			}
		}
		sum := sha256.Sum256([]byte(sessionKey(s)))
		p.Tag = "cd-" + hex.EncodeToString(sum[:])[:12]
		p.URL = "/"
		if pushURLIDRe.MatchString(s.SessionID) {
			p.URL = "/?s=" + s.SessionID
		}
	}
	return encodePush(p)
}

func (n *Notifier) groupPayload(list []agents.Session) []byte {
	count := fmt.Sprintf("%d sesiones te esperan", len(list))
	p := pushPayload{V: 1, Tag: "cd-group", URL: "/"}
	if n.mode() == PushContentNone {
		p.Title, p.Body, p.Tag = "claude-dashboard", count, "claude-dashboard"
		return encodePush(p)
	}
	p.Title = count
	seen := map[string]bool{}
	var projects []string
	for i := range list {
		if pr := oneLine(list[i].Project); pr != "" && !seen[pr] {
			seen[pr] = true
			projects = append(projects, pr)
		}
	}
	sort.Strings(projects)
	p.Body = truncRunes(strings.Join(projects, ", "), maxPushRunes)
	return encodePush(p)
}

// encodePush serializa y garantiza ≤ maxPushBytes recortando body y, si hace
// falta, title (un runa no-BMP escapada pesa hasta 12 bytes en JSON).
func encodePush(p pushPayload) []byte {
	for {
		var buf bytes.Buffer
		enc := json.NewEncoder(&buf)
		enc.SetEscapeHTML(false)
		if err := enc.Encode(p); err != nil {
			return nil
		}
		b := bytes.TrimRight(buf.Bytes(), "\n")
		if len(b) <= maxPushBytes {
			return b
		}
		switch {
		case p.Body != "":
			p.Body = truncRunes(p.Body, len([]rune(p.Body))/2)
			if p.Body == "…" {
				p.Body = ""
			}
		case p.Title != "":
			p.Title = truncRunes(p.Title, len([]rune(p.Title))/2)
			if p.Title == "…" {
				p.Title = ""
			}
		default:
			return nil
		}
	}
}

// ---- /api/push/* ----

// pushEnabled: rutas de push solo con el modo remoto completo y claves VAPID.
func (s *Server) pushEnabled() bool {
	return s.remoteEnabled() && s.Subs != nil && s.Keys != nil
}

func (s *Server) registerPush(mux *http.ServeMux) {
	if !s.pushEnabled() {
		return
	}
	mux.HandleFunc("GET /api/push/key", s.handlePushKey)
	mux.HandleFunc("POST /api/push/subscribe", s.handlePushSubscribe)
	mux.HandleFunc("POST /api/push/unsubscribe", s.handlePushUnsubscribe)
}

func (s *Server) handlePushKey(w http.ResponseWriter, r *http.Request) {
	writeJSONStatus(w, http.StatusOK, map[string]string{"key": s.Keys.PublicKeyB64()})
}

func b64Len(v string, want int) bool {
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(v, "="))
	return err == nil && len(raw) == want
}

func (s *Server) handlePushSubscribe(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Endpoint string `json:"endpoint"`
		Keys     struct {
			P256dh string `json:"p256dh"`
			Auth   string `json:"auth"`
		} `json:"keys"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 8<<10)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONStatus(w, http.StatusBadRequest, map[string]string{"error": "bad request"})
		return
	}
	// Se valida acá para distinguir 400 (entrada) de 500 (disco).
	if err := remote.ValidateEndpoint(req.Endpoint); err != nil || !b64Len(req.Keys.P256dh, 65) || !b64Len(req.Keys.Auth, 16) {
		writeJSONStatus(w, http.StatusBadRequest, map[string]string{"error": "invalid subscription"})
		return
	}
	dev := deviceID(r)
	if dev == "" {
		dev = localDeviceID
	}
	if err := s.Subs.Add(dev, req.Endpoint, req.Keys.P256dh, req.Keys.Auth); err != nil {
		s.logf("push/subscribe: %v", err)
		writeJSONStatus(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}
	// Carrera con la revocación: si el dispositivo se borró entre el gate y Add,
	// no dejamos una suscripción huérfana.
	if dev != localDeviceID && !s.Devices.Has(dev) {
		if _, err := s.Subs.RemoveDevice(dev); err != nil {
			s.logf("push/subscribe: limpiando %s: %v", dev, err)
		}
		writeJSONStatus(w, http.StatusUnauthorized, map[string]string{"error": "unpaired"})
		return
	}
	writeJSONStatus(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handlePushUnsubscribe(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Endpoint string `json:"endpoint"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4<<10)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Endpoint == "" {
		writeJSONStatus(w, http.StatusBadRequest, map[string]string{"error": "bad request"})
		return
	}
	dev := deviceID(r)
	// Un dispositivo remoto solo borra sus propias suscripciones.
	if dev != "" {
		owned := false
		for _, sub := range s.Subs.All() {
			if sub.Endpoint == req.Endpoint && sub.DeviceID == dev {
				owned = true
				break
			}
		}
		if !owned {
			w.WriteHeader(http.StatusNoContent)
			return
		}
	}
	if _, err := s.Subs.Remove(req.Endpoint); err != nil {
		s.logf("push/unsubscribe: %v", err)
		writeJSONStatus(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
