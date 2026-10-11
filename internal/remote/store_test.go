package remote

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

type rec struct {
	N    int    `json:"n"`
	Text string `json:"text"`
}

func TestWriteJSONModes(t *testing.T) {
	root := t.TempDir()
	st, err := NewStore(filepath.Join(root, "estado"))
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Write("devices.json", rec{N: 1, Text: "x"}); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(st.Path("devices.json"))
	if err != nil || fi.Mode().Perm() != 0o600 {
		t.Fatalf("archivo: %v %v", fi, err)
	}
	di, err := os.Stat(filepath.Join(root, "estado"))
	if err != nil || di.Mode().Perm() != 0o700 {
		t.Fatalf("dir: %v %v", di, err)
	}
	var got rec
	if err := st.Read("devices.json", &got); err != nil || got != (rec{1, "x"}) {
		t.Fatalf("read: %+v %v", got, err)
	}
	// sin temporales sobrantes
	ents, _ := os.ReadDir(filepath.Join(root, "estado"))
	if len(ents) != 1 {
		t.Fatalf("sobraron archivos: %v", ents)
	}
}

func TestReadFixesLoosePermsAndMissingIsZero(t *testing.T) {
	dir := t.TempDir()
	st, err := NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	var zero rec
	if err := st.Read("nada.json", &zero); err != nil || zero != (rec{}) {
		t.Fatalf("inexistente: %+v %v", zero, err)
	}
	p := filepath.Join(dir, "subs.json")
	if err := os.WriteFile(p, []byte(`{"n":7,"text":"y"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(p, 0o644); err != nil { // por si el umask recortó
		t.Fatal(err)
	}
	var got rec
	if err := st.Read("subs.json", &got); err != nil || got.N != 7 {
		t.Fatalf("read: %+v %v", got, err)
	}
	fi, _ := os.Stat(p)
	if fi.Mode().Perm() != 0o600 {
		t.Fatalf("modo = %v; quería 0600", fi.Mode().Perm())
	}
}

func TestDirDefaultUsesUserConfigDir(t *testing.T) {
	cfg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfg)
	got, err := Dir("")
	if err != nil {
		t.Skipf("sin UserConfigDir: %v", err)
	}
	if want := filepath.Join(cfg, "claude-dashboard"); got != want {
		t.Fatalf("Dir = %q; quería %q", got, want)
	}
}

func TestConcurrentWritesNoCorruption(t *testing.T) {
	st, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				if err := st.Write("c.json", rec{N: i*100 + j, Text: fmt.Sprint(i)}); err != nil {
					t.Errorf("write: %v", err)
					return
				}
				var r rec
				if err := st.Read("c.json", &r); err != nil {
					t.Errorf("read: %v", err)
					return
				}
			}
		}(i)
	}
	wg.Wait()
	// también writeJSON directo (sin mutex del store) debe ser atómico
	p := st.Path("d.json")
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				if err := writeJSON(p, rec{N: i, Text: "d"}); err != nil {
					t.Errorf("writeJSON: %v", err)
					return
				}
				var r rec
				if err := readJSON(p, &r); err != nil {
					t.Errorf("readJSON: %v", err)
					return
				}
			}
		}(i)
	}
	wg.Wait()
	var r rec
	if err := st.Read("c.json", &r); err != nil || r.Text == "" {
		t.Fatalf("final: %+v %v", r, err)
	}
}
