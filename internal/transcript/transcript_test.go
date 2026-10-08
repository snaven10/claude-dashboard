package transcript

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func setup(t *testing.T, root string) {
	t.Helper()
	old := Root
	Root = root
	mu.Lock()
	cache = map[string]cacheEntry{}
	mu.Unlock()
	bytesRead.Store(0)
	readCalls.Store(0)
	t.Cleanup(func() { Root = old })
}

func TestSummaryBasic(t *testing.T) {
	setup(t, "testdata")
	d, err := Summary("/work/demo", "sess-basic")
	if err != nil {
		t.Fatal(err)
	}
	if d.Title != "Arreglar login" {
		t.Errorf("title = %q", d.Title)
	}
	if d.LastPrompt != "arreglá el <b>bug</b> de login" {
		t.Errorf("lastPrompt = %q (tool_result, isMeta y avisos del harness no son prompt real)", d.LastPrompt)
	}
	if d.LastAction == nil || d.LastAction.Kind != "tool" || d.LastAction.Text != "Bash: go test ./..." {
		t.Errorf("lastAction = %+v", d.LastAction)
	}
	want := time.Date(2026, 1, 1, 10, 1, 20, 0, time.UTC).UnixMilli()
	if d.LastActivityAt != want {
		t.Errorf("lastActivityAt = %d, want %d", d.LastActivityAt, want)
	}
}

func TestSummaryGlobFallbackAndTextAction(t *testing.T) {
	setup(t, "testdata")
	d, err := Summary("/cwd/que/no/coincide", "sess-glob")
	if err != nil {
		t.Fatal(err)
	}
	if d.LastPrompt != "prompt via glob" {
		t.Errorf("lastPrompt = %q", d.LastPrompt)
	}
	if d.LastAction == nil || d.LastAction.Kind != "text" || d.LastAction.Text != "Listo, terminé." {
		t.Errorf("lastAction = %+v", d.LastAction)
	}
}

func TestSummaryMissing(t *testing.T) {
	setup(t, "testdata")
	d, err := Summary("/work/demo", "no-existe")
	if err != nil {
		t.Fatalf("no debe fallar: %v", err)
	}
	if d != (Detail{}) {
		t.Errorf("esperaba Detail vacío, got %+v", d)
	}
}

func TestSlug(t *testing.T) {
	if got := slugFor("/home/u/my.proj"); got != "-home-u-my-proj" {
		t.Errorf("slug = %q", got)
	}
}

func userLine(text string) string {
	return fmt.Sprintf(`{"type":"user","timestamp":"2026-03-03T00:00:00Z","message":{"role":"user","content":%q}}`, text)
}

func toolResultLine() string {
	return `{"type":"user","timestamp":"2026-03-03T00:00:01Z","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"x","content":"` + strings.Repeat("r", 200) + `"}]}}`
}

// Archivo de 50 MB: solo se lee la cola (≤ 2 MB).
func TestBigFileReadsAtMost2MB(t *testing.T) {
	root := t.TempDir()
	setup(t, root)
	dir := filepath.Join(root, "-big")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(filepath.Join(dir, "big.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	// El prompt real está al principio de la cola "útil": 1 MB de tool_results después.
	filler := toolResultLine() + "\n"
	for i := 0; i < (49<<20)/len(filler)+1; i++ {
		f.WriteString(filler)
	}
	f.WriteString(userLine("prompt enterrado") + "\n")
	for i := 0; i < (1<<20)/len(filler)+1; i++ {
		f.WriteString(filler)
	}
	f.Close()
	st, _ := os.Stat(f.Name())
	if st.Size() < 50<<20 {
		t.Fatalf("archivo chico: %d", st.Size())
	}

	d, err := Summary("/big", "big")
	if err != nil {
		t.Fatal(err)
	}
	if d.LastPrompt != "prompt enterrado" {
		t.Errorf("lastPrompt = %q (debió ampliar la ventana)", d.LastPrompt)
	}
	if got := bytesRead.Load(); got > maxWindow {
		t.Errorf("leyó %d bytes, máximo %d", got, maxWindow)
	}
	if readCalls.Load() < 2 {
		t.Errorf("esperaba ampliar la ventana, reads = %d", readCalls.Load())
	}
}

func TestBigFileWithoutRealUserStopsAt2MB(t *testing.T) {
	root := t.TempDir()
	setup(t, root)
	dir := filepath.Join(root, "-big2")
	os.MkdirAll(dir, 0o755)
	var sb strings.Builder
	filler := toolResultLine() + "\n"
	for sb.Len() < 6<<20 {
		sb.WriteString(filler)
	}
	if err := os.WriteFile(filepath.Join(dir, "s.jsonl"), []byte(sb.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	d, err := Summary("/big2", "s")
	if err != nil {
		t.Fatal(err)
	}
	if d.LastPrompt != "" {
		t.Errorf("lastPrompt = %q", d.LastPrompt)
	}
	if got := bytesRead.Load(); got > maxWindow {
		t.Errorf("leyó %d bytes, máximo %d", got, maxWindow)
	}
}

func TestCache(t *testing.T) {
	root := t.TempDir()
	setup(t, root)
	dir := filepath.Join(root, "-c")
	os.MkdirAll(dir, 0o755)
	p := filepath.Join(dir, "s.jsonl")
	os.WriteFile(p, []byte(userLine("uno")+"\n"), 0o644)

	d1, _ := Summary("/c", "s")
	calls := readCalls.Load()
	d2, _ := Summary("/c", "s")
	if readCalls.Load() != calls {
		t.Errorf("segunda llamada releyó el archivo")
	}
	if d1 != d2 {
		t.Errorf("detalle distinto: %+v vs %+v", d1, d2)
	}

	// Cambia el archivo (size+mtime) → relee.
	os.WriteFile(p, []byte(userLine("uno")+"\n"+userLine("dos")+"\n"), 0o644)
	future := time.Now().Add(time.Hour)
	os.Chtimes(p, future, future)
	d3, _ := Summary("/c", "s")
	if readCalls.Load() == calls {
		t.Errorf("no releyó tras cambiar el archivo")
	}
	if d3.LastPrompt != "dos" {
		t.Errorf("lastPrompt = %q", d3.LastPrompt)
	}
}

func TestTruncate(t *testing.T) {
	if got := truncate(strings.Repeat("á", 600), maxPrompt); len([]rune(got)) != maxPrompt {
		t.Errorf("len = %d", len([]rune(got)))
	}
}
