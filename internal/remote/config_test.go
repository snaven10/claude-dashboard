package remote

import "testing"

func TestParsePublicURL(t *testing.T) {
	const want = "https://mi-pc.tailnet-ejemplo.ts.net"
	for _, in := range []string{want, want + "/", " " + want + " "} {
		got, err := ParsePublicURL(in)
		if err != nil || got != want {
			t.Errorf("ParsePublicURL(%q) = %q, %v; quería %q", in, got, err, want)
		}
	}
	for _, in := range []string{
		"", "http://mi-pc.tailnet-ejemplo.ts.net", "https://", "https:///x",
		want + "/panel", want + "?a=1", want + "/?", want + "/#pair=x",
		"https://user:pw@mi-pc.tailnet-ejemplo.ts.net", "mi-pc.tailnet-ejemplo.ts.net", "ftp://x.test",
	} {
		if got, err := ParsePublicURL(in); err == nil {
			t.Errorf("ParsePublicURL(%q) = %q; quería error", in, got)
		}
	}
}

func TestValidateRemoteAddr(t *testing.T) {
	for _, ok := range []string{"127.0.0.1:7778", "[::1]:7778", "localhost:7778"} {
		if err := ValidateRemoteAddr(ok); err != nil {
			t.Errorf("ValidateRemoteAddr(%q): %v", ok, err)
		}
	}
	for _, bad := range []string{"0.0.0.0:7778", "192.0.2.1:7778", ":7778", "7778", "", "[::]:7778"} {
		if err := ValidateRemoteAddr(bad); err == nil {
			t.Errorf("ValidateRemoteAddr(%q): quería error", bad)
		}
	}
}
