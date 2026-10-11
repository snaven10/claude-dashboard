package webpush

// Envío de un push cifrado al servicio de push del navegador (RFC 8030 +
// RFC 8292). Decisión: DD-51 de PLAN-011.

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	// QueueSize es el tope de la cola de envíos (DD-51).
	QueueSize = 64

	pushTTL       = "600"
	maxTopicLen   = 32
	defaultRetry  = 60 * time.Second // 429/5xx sin Retry-After
	maxRetryAfter = time.Hour        // un Retry-After absurdo no silencia un host para siempre
	clientTimeout = 10 * time.Second
)

// Subscription es lo que el servicio de push necesita: endpoint y claves del
// navegador en b64url sin padding.
type Subscription struct {
	Endpoint string
	P256dh   string
	Auth     string
}

// Outcome es la clase de resultado de un envío.
type Outcome int

const (
	// OK: el servicio aceptó el aviso (201/202).
	OK Outcome = iota
	// Gone: la suscripción ya no existe (404/410); hay que borrarla.
	Gone
	// RetryAfter: ese host pidió esperar (429/5xx) o todavía está en pausa;
	// el aviso NO se reintenta. Result.RetryAt dice hasta cuándo.
	RetryAfter
	// Err: cualquier otro fallo (413, 4xx, red, cifrado). La suscripción se conserva.
	Err
)

func (o Outcome) String() string {
	switch o {
	case OK:
		return "ok"
	case Gone:
		return "gone"
	case RetryAfter:
		return "retry-after"
	default:
		return "error"
	}
}

// Result es el resultado tipado de Send.
type Result struct {
	Outcome Outcome
	Status  int       // HTTP, 0 si no hubo respuesta
	RetryAt time.Time // solo con RetryAfter
	Err     error     // solo con Err
}

// TopicFor devuelve el Topic de un aviso: 22 chars b64url de los primeros 16
// bytes del SHA-256 del sessionId (un aviso nuevo de la misma sesión reemplaza
// al no entregado).
func TopicFor(sessionID string) string {
	sum := sha256.Sum256([]byte(sessionID))
	return b64url.EncodeToString(sum[:16])
}

type job struct {
	sub     Subscription
	payload []byte
	topic   string
}

// Sender manda avisos. Send es síncrono; Enqueue + Start dan la cola acotada
// con un worker. Seguro para uso concurrente.
type Sender struct {
	Client *http.Client // nil = cliente con guard SSRF, sin redirects ni proxy, 10s
	Keys   *Keys
	Now    func() time.Time // nil = time.Now
	// allowPrivate desactiva el guard de IPs no públicas del cliente por
	// defecto. No exportado: solo los tests del paquete lo setean.
	allowPrivate bool

	// OnGone se llama con el endpoint cuando el servicio responde 404/410.
	OnGone func(endpoint string)

	defOnce   sync.Once
	defClient *http.Client

	once  sync.Once
	queue chan job
	wg    sync.WaitGroup

	mu    sync.Mutex
	until map[string]time.Time // host → no enviar antes de
}

func (s *Sender) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

func (s *Sender) client() *http.Client {
	// Siempre sin seguir redirecciones (SSRF), también con Client inyectado.
	if s.Client != nil {
		c := *s.Client
		c.CheckRedirect = noRedirect
		return &c
	}
	s.defOnce.Do(func() { s.defClient = newGuardedClient(s.allowPrivate) })
	return s.defClient
}

func (s *Sender) init() {
	s.once.Do(func() {
		s.queue = make(chan job, QueueSize)
		s.until = map[string]time.Time{}
	})
}

// Send cifra payload para sub y lo manda. topic ≤ 32 chars b64url.
func (s *Sender) Send(ctx context.Context, sub Subscription, payload []byte, topic string) Result {
	s.init()
	if s.Keys == nil {
		return Result{Outcome: Err, Err: errors.New("webpush: sin claves VAPID")}
	}
	if err := checkTopic(topic); err != nil {
		return Result{Outcome: Err, Err: err}
	}
	u, err := url.Parse(sub.Endpoint)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return Result{Outcome: Err, Err: fmt.Errorf("webpush: endpoint inválido")}
	}
	host := strings.ToLower(u.Host)

	now := s.now()
	s.mu.Lock()
	wait, paused := s.until[host]
	s.mu.Unlock()
	if paused && now.Before(wait) {
		return Result{Outcome: RetryAfter, RetryAt: wait}
	}

	p256dh, err := b64url.DecodeString(strings.TrimRight(sub.P256dh, "="))
	if err != nil {
		return Result{Outcome: Err, Err: fmt.Errorf("webpush: p256dh: %w", err)}
	}
	auth, err := b64url.DecodeString(strings.TrimRight(sub.Auth, "="))
	if err != nil {
		return Result{Outcome: Err, Err: fmt.Errorf("webpush: auth: %w", err)}
	}
	body, err := Encrypt(payload, p256dh, auth)
	if err != nil {
		return Result{Outcome: Err, Err: err}
	}
	jwt, err := s.Keys.JWT(u.Scheme+"://"+u.Host, now)
	if err != nil {
		return Result{Outcome: Err, Err: err}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, sub.Endpoint, bytes.NewReader(body))
	if err != nil {
		return Result{Outcome: Err, Err: fmt.Errorf("webpush: construyendo petición: %w", err)}
	}
	req.Header.Set("Authorization", "vapid t="+jwt+", k="+s.Keys.PublicKeyB64())
	req.Header.Set("Content-Encoding", "aes128gcm")
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("TTL", pushTTL)
	req.Header.Set("Urgency", "high")
	if topic != "" {
		req.Header.Set("Topic", topic)
	}

	resp, err := s.client().Do(req)
	if err != nil {
		return Result{Outcome: Err, Err: fmt.Errorf("webpush: enviando: %w", err)}
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))

	st := resp.StatusCode
	switch {
	case st >= 300 && st < 400:
		return Result{Outcome: Err, Status: st, Err: fmt.Errorf("webpush: redirección %d no seguida", st)}
	case st == http.StatusCreated || st == http.StatusAccepted:
		return Result{Outcome: OK, Status: st}
	case st == http.StatusNotFound || st == http.StatusGone:
		return Result{Outcome: Gone, Status: st}
	case st == http.StatusTooManyRequests || st >= 500:
		at := now.Add(parseRetryAfter(resp.Header.Get("Retry-After"), now))
		s.mu.Lock()
		s.until[host] = at
		s.mu.Unlock()
		return Result{Outcome: RetryAfter, Status: st, RetryAt: at}
	default:
		// 413 = bug de tamaño (el notificador trunca antes); el resto, log.
		return Result{Outcome: Err, Status: st, Err: fmt.Errorf("webpush: el servicio respondió %d", st)}
	}
}

func checkTopic(topic string) error {
	if len(topic) > maxTopicLen {
		return fmt.Errorf("webpush: topic de %d chars, máximo %d", len(topic), maxTopicLen)
	}
	for _, r := range topic {
		ok := r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_'
		if !ok {
			return errors.New("webpush: topic con caracteres fuera de b64url")
		}
	}
	return nil
}

// parseRetryAfter acepta segundos o HTTP-date; sin valor válido = 60 s.
func parseRetryAfter(v string, now time.Time) time.Duration {
	v = strings.TrimSpace(v)
	d := defaultRetry
	if secs, err := strconv.Atoi(v); err == nil && secs >= 0 {
		d = time.Duration(secs) * time.Second
	} else if t, err := http.ParseTime(v); err == nil {
		d = t.Sub(now)
	}
	if d < 0 {
		d = 0
	}
	if d > maxRetryAfter {
		d = maxRetryAfter
	}
	return d
}

// Enqueue encola un envío sin bloquear nunca. Devuelve false (y loguea) si la
// cola está llena: el aviso se descarta.
func (s *Sender) Enqueue(sub Subscription, payload []byte, topic string) bool {
	s.init()
	select {
	case s.queue <- job{sub: sub, payload: payload, topic: topic}:
		return true
	default:
		log.Printf("webpush: cola llena (%d); aviso descartado", QueueSize)
		return false
	}
}

// Start lanza el worker que vacía la cola; termina cuando ctx se cancela.
// Wait espera a que salga. Llamarlo más de una vez lanza más workers: no lo hagas.
func (s *Sender) Start(ctx context.Context) {
	s.init()
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			case j := <-s.queue:
				s.handle(ctx, j)
			}
		}
	}()
}

// Wait bloquea hasta que el worker terminó (tras cancelar el ctx de Start).
func (s *Sender) Wait() { s.wg.Wait() }

func (s *Sender) handle(ctx context.Context, j job) {
	res := s.Send(ctx, j.sub, j.payload, j.topic)
	switch res.Outcome {
	case OK:
	case Gone:
		if s.OnGone != nil {
			s.OnGone(j.sub.Endpoint)
		}
	case RetryAfter:
		log.Printf("webpush: servicio de push pidió esperar (status %d) hasta %s; aviso descartado",
			res.Status, res.RetryAt.Format(time.RFC3339))
	default:
		log.Printf("webpush: envío falló (status %d): %v", res.Status, res.Err)
	}
}
