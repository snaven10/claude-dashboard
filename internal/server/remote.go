package server

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/url"
	"strings"

	"claude-dashboard/internal/remote"
)

type deviceKey struct{}

// remoteEnabled: modo remoto completo (DD-44). Sin esto no se registra ninguna
// ruta nueva.
func (s *Server) remoteEnabled() bool {
	return s.PublicURL != "" && s.Pairing != nil && s.Devices != nil
}

// registerRemote agrega las rutas de DD-47. Solo con el modo remoto activo.
func (s *Server) registerRemote(mux *http.ServeMux) {
	if !s.remoteEnabled() {
		return
	}
	mux.HandleFunc("GET /api/remote", s.handleRemote)
	mux.HandleFunc("POST /api/pair/new", s.handlePairNew)
	mux.HandleFunc("POST /api/pair/redeem", s.handlePairRedeem)
	mux.HandleFunc("GET /api/devices", s.handleDevicesList)
	mux.HandleFunc("DELETE /api/devices/{id}", s.handleDeviceRevoke)
}

func writeJSONStatus(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

// gate envuelve el mux de ambos listeners (DD-47). Falla cerrado: un listener
// remoto sin el modo remoto completo no sirve NINGUNA ruta /api/.
func (s *Server) gate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		isAPI := strings.HasPrefix(p, "/api/")
		rem := isRemote(r)

		if rem && isAPI && !s.remoteEnabled() {
			http.NotFound(w, r)
			return
		}
		if !s.remoteEnabled() {
			next.ServeHTTP(w, r)
			return
		}

		if rem && (p == "/api/pair/new" || p == "/api/devices" || strings.HasPrefix(p, "/api/devices/")) {
			http.NotFound(w, r)
			return
		}
		if !rem && p == "/api/pair/redeem" {
			http.NotFound(w, r)
			return
		}
		if isAPI && (r.Method == http.MethodPost || r.Method == http.MethodDelete) {
			if !s.sameOriginJSON(r, rem) {
				writeJSONStatus(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
				return
			}
		}
		if rem && isAPI && p != "/api/remote" && p != "/api/pair/redeem" {
			dev, ok := s.deviceFromRequest(r)
			if !ok {
				writeJSONStatus(w, http.StatusUnauthorized, map[string]string{"error": "unpaired"})
				return
			}
			s.Devices.Touch(dev.ID)
			r = r.WithContext(context.WithValue(r.Context(), deviceKey{}, dev.ID))
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) deviceFromRequest(r *http.Request) (remote.Device, bool) {
	c, err := r.Cookie(remote.CookieName)
	if err != nil {
		return remote.Device{}, false
	}
	return s.Devices.Lookup(c.Value)
}

// deviceID devuelve el dispositivo autenticado de la petición ("" en local).
func deviceID(r *http.Request) string {
	v, _ := r.Context().Value(deviceKey{}).(string)
	return v
}

// sameOriginJSON implementa DD-48: Content-Type JSON, Origin propio si viene y
// Sec-Fetch-Site same-origin/none si viene.
func (s *Server) sameOriginJSON(r *http.Request, rem bool) bool {
	ct := r.Header.Get("Content-Type")
	if i := strings.IndexByte(ct, ';'); i >= 0 {
		ct = ct[:i]
	}
	if !strings.EqualFold(strings.TrimSpace(ct), "application/json") {
		return false
	}
	if sfs := r.Header.Get("Sec-Fetch-Site"); sfs != "" && sfs != "same-origin" && sfs != "none" {
		return false
	}
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	if rem {
		return origin == s.PublicURL
	}
	u, err := url.Parse(origin)
	if err != nil || u.Scheme != "http" {
		return false
	}
	if h := u.Hostname(); h != "127.0.0.1" && h != "localhost" {
		return false
	}
	la, ok := r.Context().Value(http.LocalAddrContextKey).(net.Addr)
	if !ok {
		return false
	}
	_, port, err := net.SplitHostPort(la.String())
	return err == nil && u.Port() == port
}

func (s *Server) handleRemote(w http.ResponseWriter, r *http.Request) {
	_, paired := s.deviceFromRequest(r)
	writeJSONStatus(w, http.StatusOK, struct {
		Enabled bool `json:"enabled"`
		Local   bool `json:"local"`
		Paired  bool `json:"paired"`
	}{true, !isRemote(r), paired})
}

func (s *Server) handlePairNew(w http.ResponseWriter, r *http.Request) {
	token, code, exp, err := s.Pairing.New()
	if err != nil {
		s.logf("pair/new: %v", err)
		writeJSONStatus(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}
	writeJSONStatus(w, http.StatusOK, struct {
		URL       string `json:"url"`
		Code      string `json:"code"`
		ExpiresAt int64  `json:"expiresAt"`
	}{s.PublicURL + "/#pair=" + token, code, exp.UnixMilli()})
}

func (s *Server) handlePairRedeem(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token string `json:"token"`
		Code  string `json:"code"`
		Name  string `json:"name"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4<<10)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONStatus(w, http.StatusBadRequest, map[string]string{"error": "bad request"})
		return
	}
	if err := s.Pairing.Redeem(req.Token, req.Code); err != nil {
		// Vencido e inválido se responden igual: no es un oráculo.
		writeJSONStatus(w, http.StatusUnauthorized, map[string]string{"error": "invalid"})
		return
	}
	name := req.Name
	if strings.TrimSpace(name) == "" {
		name = nameFromUA(r.UserAgent())
	}
	cookie, dev, err := s.Devices.Create(name, r.Header.Get("Tailscale-User-Login"))
	if err != nil {
		s.logf("pair/redeem: %v", err)
		writeJSONStatus(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     remote.CookieName,
		Value:    cookie,
		Path:     "/",
		MaxAge:   remote.CookieMaxAge,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	})
	writeJSONStatus(w, http.StatusOK, map[string]string{"deviceId": dev.ID})
}

func nameFromUA(ua string) string {
	switch {
	case strings.Contains(ua, "iPhone"):
		return "iPhone"
	case strings.Contains(ua, "iPad"):
		return "iPad"
	case strings.Contains(ua, "Android"):
		return "Android"
	}
	return "dispositivo"
}

func (s *Server) handleDevicesList(w http.ResponseWriter, r *http.Request) {
	writeJSONStatus(w, http.StatusOK, struct {
		Devices []remote.Device `json:"devices"`
	}{s.Devices.List()})
}

func (s *Server) handleDeviceRevoke(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	ok, err := s.Devices.Revoke(id)
	if err != nil {
		s.logf("devices/revoke: %v", err)
		writeJSONStatus(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}
	if !ok {
		writeJSONStatus(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	// DD-49: revocar borra también las suscripciones push del dispositivo.
	if s.Subs != nil {
		if _, err := s.Subs.RemoveDevice(id); err != nil {
			s.logf("devices/revoke: borrando suscripciones de %s: %v", id, err)
		}
	}
	s.closeDevice(id)
	w.WriteHeader(http.StatusNoContent)
}

// closeDevice corta los SSE abiertos del dispositivo (DD-49). El close lo hace
// siempre quien borra la entrada de s.subs bajo s.mu: un solo dueño.
func (s *Server) closeDevice(id string) {
	if id == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for ch, dev := range s.subDev {
		if dev == id {
			s.dropLocked(ch)
		}
	}
}
