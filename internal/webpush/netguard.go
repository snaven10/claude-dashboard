package webpush

// Defensa SSRF del envío (DD-51): el endpoint de una suscripción lo manda el
// teléfono, así que el daemon no puede confiar en que apunte a Internet.

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"syscall"
)

var (
	cgnatPrefix = netip.MustParsePrefix("100.64.0.0/10")
	ulaPrefix   = netip.MustParsePrefix("fc00::/7")
)

// errBlockedAddr es el error del dialer cuando la IP efectiva no es pública.
var errBlockedAddr = errors.New("webpush: destino no público bloqueado")

// isPublicAddr dice si ip es un destino público permitido. Bloquea loopback,
// privadas (RFC1918), link-local, CGNAT (tailnet), ULA, no especificada y
// multicast. IPv4 mapeada en IPv6 se evalúa como IPv4.
func isPublicAddr(ip netip.Addr) bool {
	ip = ip.Unmap().WithZone("")
	if !ip.IsValid() {
		return false
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified() ||
		ip.IsInterfaceLocalMulticast() {
		return false
	}
	if cgnatPrefix.Contains(ip) || ulaPrefix.Contains(ip) {
		return false
	}
	return true
}

// guardControl se usa como net.Dialer.Control: corre con la IP ya resuelta,
// justo antes de conectar, así que cubre DNS rebinding.
func guardControl(_, address string, _ syscall.RawConn) error {
	ap, err := netip.ParseAddrPort(address)
	if err != nil {
		return fmt.Errorf("%w: %v", errBlockedAddr, err)
	}
	if !isPublicAddr(ap.Addr()) {
		return errBlockedAddr
	}
	return nil
}

// noRedirect: un 3xx se devuelve tal cual (no se sigue).
func noRedirect(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }

// newGuardedClient arma el cliente por defecto. allowPrivate solo lo setean
// los tests del paquete; producción siempre bloquea.
func newGuardedClient(allowPrivate bool) *http.Client {
	d := &net.Dialer{Timeout: clientTimeout}
	if !allowPrivate {
		d.Control = guardControl
	}
	return &http.Client{
		Timeout:       clientTimeout,
		CheckRedirect: noRedirect,
		Transport: &http.Transport{
			Proxy:               nil, // sin HTTPS_PROXY: el guard no se saltea
			DialContext:         d.DialContext,
			TLSHandshakeTimeout: clientTimeout,
		},
	}
}
