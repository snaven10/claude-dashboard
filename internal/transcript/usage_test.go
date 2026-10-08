package transcript

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func asstLine(id string, in, out, cr, cc int, model string) string {
	return fmt.Sprintf(`{"type":"assistant","timestamp":"2026-01-01T10:00:00.000Z","message":{"id":%q,"model":%q,"role":"assistant","content":[{"type":"text","text":"x"}],"usage":{"input_tokens":%d,"output_tokens":%d,"cache_read_input_tokens":%d,"cache_creation_input_tokens":%d}}}`+"\n",
		id, model, in, out, cr, cc)
}

func usageSetup(t *testing.T) (root, path string) {
	t.Helper()
	root = t.TempDir()
	old := Root
	Root = root
	usageBytesRead.Store(0)
	bytesRead.Store(0)
	t.Cleanup(func() { Root = old })
	dir := filepath.Join(root, "-work-usage")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	return root, filepath.Join(dir, "sess-u.jsonl")
}

func appendFile(t *testing.T, path, s string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := f.WriteString(s); err != nil {
		t.Fatal(err)
	}
}

func TestUsageDedupsSplitMessage(t *testing.T) {
	_, path := usageSetup(t)
	// un mensaje partido en 3 líneas con el mismo message.id y el mismo usage
	var b strings.Builder
	for i := 0; i < 3; i++ {
		b.WriteString(asstLine("msg_1", 10, 20, 300, 40, "claude-opus-5-5"))
	}
	b.WriteString(asstLine("msg_2", 1, 2, 3, 4, "claude-opus-5-5"))
	if err := os.WriteFile(path, []byte(b.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	tk, err := Usage("/work/usage", "sess-u")
	if err != nil || tk == nil {
		t.Fatalf("tk=%v err=%v", tk, err)
	}
	want := Tokens{Input: 11, Output: 22, CacheRead: 303, CacheCreation: 44, Model: "claude-opus-5-5"}
	if *tk != want {
		t.Errorf("got %+v want %+v", *tk, want)
	}
}

func TestUsageChunkBoundaries(t *testing.T) {
	_, path := usageSetup(t)
	old := usageChunk
	usageChunk = 7 // casi todas las líneas quedan partidas entre bloques
	t.Cleanup(func() { usageChunk = old })
	var b strings.Builder
	for i := 0; i < 3; i++ {
		b.WriteString(asstLine("msg_1", 10, 20, 300, 40, "claude-opus-5-5"))
	}
	b.WriteString(asstLine("msg_2", 1, 2, 3, 4, "claude-opus-5-5"))
	b.WriteString(`{"type":"assistant","message":{"id":"msg_3","usage":{"output_tokens":5`) // incompleta
	if err := os.WriteFile(path, []byte(b.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	tk, err := Usage("/work/usage", "sess-u")
	if err != nil || tk == nil {
		t.Fatalf("tk=%v err=%v", tk, err)
	}
	want := Tokens{Input: 11, Output: 22, CacheRead: 303, CacheCreation: 44, Model: "claude-opus-5-5"}
	if *tk != want {
		t.Errorf("got %+v want %+v", *tk, want)
	}
	// al completarse, la línea partida se cuenta una sola vez
	appendFile(t, path, `}}}`+"\n")
	tk, _ = Usage("/work/usage", "sess-u")
	if tk == nil || tk.Output != 27 {
		t.Errorf("tras completar la línea: %+v, esperaba output 27", tk)
	}
}

func TestUsageIncrementalAndTruncate(t *testing.T) {
	_, path := usageSetup(t)
	first := asstLine("m1", 5, 5, 0, 0, "mod-a") + asstLine("m2", 5, 5, 0, 0, "mod-a")
	appendFile(t, path, first)
	if tk, err := Usage("/work/usage", "sess-u"); err != nil || tk == nil || tk.Input != 10 {
		t.Fatalf("1er ciclo: %v %v", tk, err)
	}
	if got := usageBytesRead.Load(); got != int64(len(first)) {
		t.Fatalf("1er ciclo leyó %d, quiero %d", got, len(first))
	}

	// 2º ciclo sin cambios: no lee nada
	if _, err := Usage("/work/usage", "sess-u"); err != nil {
		t.Fatal(err)
	}
	if got := usageBytesRead.Load(); got != int64(len(first)) {
		t.Fatalf("ciclo sin cambios leyó de más: %d", got)
	}

	// 2º ciclo con datos nuevos + línea incompleta: lee solo lo nuevo
	more := asstLine("m3", 7, 0, 0, 0, "mod-b")
	partial := `{"type":"assistant","message":{"id":"m4","usage":{"input_`
	appendFile(t, path, more+partial)
	tk, err := Usage("/work/usage", "sess-u")
	if err != nil || tk == nil {
		t.Fatalf("%v %v", tk, err)
	}
	if tk.Input != 17 || tk.Model != "mod-b" {
		t.Errorf("tras 2º ciclo: %+v", *tk)
	}
	if got, want := usageBytesRead.Load(), int64(len(first)+len(more)+len(partial)); got != want {
		t.Errorf("bytes leídos %d, quiero %d (solo lo nuevo)", got, want)
	}

	// se completa la línea parcial: se cuenta una vez
	appendFile(t, path, `tokens":3,"output_tokens":0}}}`+"\n")
	tk, _ = Usage("/work/usage", "sess-u")
	if tk.Input != 20 {
		t.Errorf("línea incompleta no se completó bien: %+v", *tk)
	}

	// truncado: se resetea
	small := asstLine("z1", 2, 0, 0, 0, "mod-c")
	if err := os.WriteFile(path, []byte(small), 0o600); err != nil {
		t.Fatal(err)
	}
	tk, _ = Usage("/work/usage", "sess-u")
	if tk == nil || tk.Input != 2 || tk.Model != "mod-c" {
		t.Errorf("tras truncar: %+v", tk)
	}
}

func TestUsageMissingAndEmpty(t *testing.T) {
	_, path := usageSetup(t)
	if tk, err := Usage("/work/usage", "no-existe"); tk != nil || err != nil {
		t.Fatalf("faltante: %v %v", tk, err)
	}
	appendFile(t, path, `{"type":"user","message":{"content":"hola"}}`+"\n")
	if tk, err := Usage("/work/usage", "sess-u"); tk != nil || err != nil {
		t.Fatalf("sin usage debe ser nil: %v %v", tk, err)
	}
}

func TestTimelineLast20NoHarness(t *testing.T) {
	_, path := usageSetup(t)
	var b strings.Builder
	b.WriteString(`{"type":"user","message":{"content":"<task-notification>x</task-notification>"}}` + "\n")
	b.WriteString(`{"type":"user","isMeta":true,"message":{"content":"meta"}}` + "\n")
	b.WriteString(`{"type":"user","message":{"content":[{"type":"tool_result","content":"ok"}]}}` + "\n")
	for i := 0; i < 15; i++ {
		fmt.Fprintf(&b, `{"type":"user","timestamp":"2026-01-01T10:%02d:00.000Z","message":{"content":"prompt %d"}}`+"\n", i, i)
		fmt.Fprintf(&b, `{"type":"assistant","timestamp":"2026-01-01T10:%02d:30.000Z","message":{"content":[{"type":"tool_use","name":"Bash","input":{"command":"go test %d"}}]}}`+"\n", i, i)
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	ev, err := Timeline("/work/usage", "sess-u")
	if err != nil {
		t.Fatal(err)
	}
	if len(ev) != 20 {
		t.Fatalf("eventos=%d, quiero 20", len(ev))
	}
	for i, e := range ev {
		if strings.Contains(e.Text, "task-notification") || e.Text == "meta" || e.Text == "ok" {
			t.Errorf("mensaje del harness en timeline: %+v", e)
		}
		if i > 0 && e.At < ev[i-1].At {
			t.Errorf("desordenado en %d", i)
		}
	}
	if last := ev[len(ev)-1]; last.Kind != "tool" || last.Text != "Bash: go test 14" {
		t.Errorf("último: %+v", last)
	}
	if ev[0].Kind != "prompt" || ev[0].Text != "prompt 5" {
		t.Errorf("primero: %+v", ev[0])
	}
}

func TestTimelineMissing(t *testing.T) {
	usageSetup(t)
	ev, err := Timeline("/work/usage", "nada")
	if err != nil || ev == nil || len(ev) != 0 {
		t.Fatalf("%v %v", ev, err)
	}
}
