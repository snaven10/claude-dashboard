// Package remote agrupa la configuración y el estado en disco del modo remoto
// (PLAN-011): validación de -public-url / -remote-addr y el store 0700/0600.
package remote

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

// ParsePublicURL valida -public-url (DD-44): https, host no vacío, sin path
// (salvo "/"), sin query ni fragmento ni credenciales. Devuelve el origen
// normalizado, sin "/" final.
func ParsePublicURL(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", fmt.Errorf("-public-url vacío")
	}
	if strings.Contains(s, "#") {
		return "", fmt.Errorf("-public-url %q no puede llevar fragmento", s)
	}
	u, err := url.Parse(s)
	if err != nil {
		return "", fmt.Errorf("-public-url %q inválido: %w", s, err)
	}
	if u.Scheme != "https" {
		return "", fmt.Errorf("-public-url %q tiene que ser https://", s)
	}
	if u.Hostname() == "" {
		return "", fmt.Errorf("-public-url %q no tiene host", s)
	}
	if u.User != nil {
		return "", fmt.Errorf("-public-url %q no puede llevar credenciales", s)
	}
	if u.Path != "" && u.Path != "/" {
		return "", fmt.Errorf("-public-url %q no puede llevar path", s)
	}
	if u.RawQuery != "" || u.ForceQuery {
		return "", fmt.Errorf("-public-url %q no puede llevar query", s)
	}
	return "https://" + u.Host, nil
}

// ValidateRemoteAddr exige loopback para -remote-addr (DD-43). No hay escape:
// -allow-remote no lo afecta.
func ValidateRemoteAddr(addr string) error {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("-remote-addr inválido %q: %w", addr, err)
	}
	if host == "localhost" {
		return nil
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		return nil
	}
	return fmt.Errorf("-remote-addr %q no es loopback: solo `tailscale serve` debe alcanzarlo", addr)
}
