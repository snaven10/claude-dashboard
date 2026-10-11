package web

import (
	"bytes"
	"encoding/json"
	"image/png"
	"testing"
)

func TestManifestValido(t *testing.T) {
	raw, err := FS.ReadFile("manifest.webmanifest")
	if err != nil {
		t.Fatal(err)
	}
	var m struct {
		Display  string `json:"display"`
		StartURL string `json:"start_url"`
		Scope    string `json:"scope"`
		Icons    []struct {
			Src     string `json:"src"`
			Sizes   string `json:"sizes"`
			Purpose string `json:"purpose"`
		} `json:"icons"`
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("manifest no es JSON válido: %v", err)
	}
	if m.Display != "standalone" || m.StartURL != "/" || m.Scope != "/" {
		t.Errorf("display/start_url/scope inesperados: %+v", m)
	}
	if len(m.Icons) != 3 {
		t.Fatalf("íconos = %d; quería 3", len(m.Icons))
	}
	maskable := 0
	for _, ic := range m.Icons {
		data, err := FS.ReadFile(ic.Src)
		if err != nil {
			t.Errorf("ícono %s no está embebido: %v", ic.Src, err)
			continue
		}
		checkPNG(t, ic.Src, data, ic.Sizes)
		if ic.Purpose == "maskable" {
			maskable++
		}
	}
	if maskable != 1 {
		t.Errorf("maskable = %d; quería 1", maskable)
	}
}

func TestAppleTouchIcon(t *testing.T) {
	data, err := FS.ReadFile("icons/apple-touch-icon.png")
	if err != nil {
		t.Fatal(err)
	}
	checkPNG(t, "apple-touch-icon.png", data, "180x180")
}

func TestSWEmbebido(t *testing.T) {
	if _, err := FS.ReadFile("sw.js"); err != nil {
		t.Fatal(err)
	}
	if _, err := FS.ReadFile("sw.test.js"); err == nil {
		t.Error("sw.test.js no debe estar embebido")
	}
}

func checkPNG(t *testing.T, name string, data []byte, sizes string) {
	t.Helper()
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Errorf("%s no decodifica: %v", name, err)
		return
	}
	b := img.Bounds()
	if got := itoa(b.Dx()) + "x" + itoa(b.Dy()); got != sizes {
		t.Errorf("%s mide %s; declara %s", name, got, sizes)
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	s := ""
	for ; n > 0; n /= 10 {
		s = string(rune('0'+n%10)) + s
	}
	return s
}
