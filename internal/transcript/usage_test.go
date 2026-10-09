package transcript

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

func asstAt(id, ts string, in, out, cr, cc int) string {
	return fmt.Sprintf(`{"type":"assistant","timestamp":%q,"message":{"id":%q,"model":"m","usage":{"input_tokens":%d,"output_tokens":%d,"cache_read_input_tokens":%d,"cache_creation_input_tokens":%d}}}`+"\n",
		ts, id, in, out, cr, cc)
}

func compactLine(ts string, pre int) string {
	return fmt.Sprintf(`{"type":"system","subtype":"compact_boundary","timestamp":%q,"compactMetadata":{"trigger":"auto","preTokens":%d,"postTokens":1000}}`+"\n", ts, pre)
}

func TestActivityCompactions(t *testing.T) {
	_, path := usageSetup(t)
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	// sin compactaciones: 0 y nil
	appendFile(t, path, asstAt("a1", "2026-01-01T10:00:00.000Z", 1, 1, 1, 1))
	c, _, err := Activity("/work/usage", "sess-u", now)
	if err != nil || c == nil {
		t.Fatalf("c=%v err=%v", c, err)
	}
	if c.Compactions != 0 || c.LastPreTokens != nil || c.LastCompactAt != nil {
		t.Errorf("sin compactar: %+v", c)
	}

	// dos compactaciones: cuenta 2 y toma las del último
	appendFile(t, path, compactLine("2026-01-01T10:30:00.000Z", 967001)+
		asstAt("a2", "2026-01-01T10:31:00.000Z", 1, 1, 1, 1)+
		compactLine("2026-01-01T11:00:00.000Z", 988000))
	c, _, _ = Activity("/work/usage", "sess-u", now)
	if c.Compactions != 2 || c.LastPreTokens == nil || *c.LastPreTokens != 988000 {
		t.Fatalf("2 compactaciones: %+v", c)
	}
	want := time.Date(2026, 1, 1, 11, 0, 0, 0, time.UTC).UnixMilli()
	if c.LastCompactAt == nil || *c.LastCompactAt != want {
		t.Errorf("lastCompactAt=%v quiero %d", c.LastCompactAt, want)
	}
}

func TestActivityContextIsLastMessageAndDedup(t *testing.T) {
	_, path := usageSetup(t)
	now := time.Date(2026, 1, 1, 10, 4, 0, 0, time.UTC)
	appendFile(t, path,
		asstAt("m1", "2026-01-01T10:00:00.000Z", 100, 5, 1000, 50)+
			asstAt("m2", "2026-01-01T10:01:00.000Z", 10, 5, 400000, 2000)+
			asstAt("m2", "2026-01-01T10:01:00.000Z", 10, 5, 400000, 2000)) // repetida
	c, r, err := Activity("/work/usage", "sess-u", now)
	if err != nil || c == nil || r == nil {
		t.Fatalf("c=%v r=%v err=%v", c, r, err)
	}
	if c.Current != 402010 {
		t.Errorf("current=%d, quiero 402010 (el del último mensaje, no la suma)", c.Current)
	}
	if r.Calls != 2 || r.Output != 10 {
		t.Errorf("rate=%+v: la repetida no debe contarse", r)
	}
}

func TestActivityRateWindow(t *testing.T) {
	_, path := usageSetup(t)
	base := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	ts := func(min float64) string {
		return base.Add(time.Duration(min * float64(time.Minute))).Format("2006-01-02T15:04:05.000Z")
	}
	appendFile(t, path,
		asstAt("old", ts(0), 1, 9000, 0, 0)+ // fuera de la ventana para now=+8
			asstAt("a", ts(4), 1, 7000, 0, 0)+
			asstAt("b", ts(6), 1, 3000, 0, 0)+
			asstAt("c", ts(7), 1, 2000, 0, 0))
	now := base.Add(8 * time.Minute) // ventana (+3, +8]
	_, r, err := Activity("/work/usage", "sess-u", now)
	if err != nil || r == nil {
		t.Fatalf("r=%v err=%v", r, err)
	}
	if r.WindowMin != 5 || r.Calls != 3 || r.Output != 12000 {
		t.Errorf("rate=%+v, quiero 3 llamadas y 12000", r)
	}
	// mucho después: sin actividad en la ventana → nil
	if _, r, _ = Activity("/work/usage", "sess-u", base.Add(time.Hour)); r != nil {
		t.Errorf("sin actividad reciente debe ser nil: %+v", r)
	}
	// mensajes de 12 min atrás se podan de la cola al llegar uno nuevo
	appendFile(t, path, asstAt("d", ts(20), 1, 1, 0, 0))
	if _, _, err := Activity("/work/usage", "sess-u", base.Add(21*time.Minute)); err != nil {
		t.Fatal(err)
	}
	st := stateFor(path)
	st.mu.Lock()
	n := len(st.recent)
	st.mu.Unlock()
	if n != 1 {
		t.Errorf("cola=%d, quiero 1 tras podar >10 min", n)
	}
}

func TestActivityIncremental(t *testing.T) {
	_, path := usageSetup(t)
	now := time.Date(2026, 1, 1, 10, 5, 0, 0, time.UTC)
	first := asstAt("a", "2026-01-01T10:00:00.000Z", 1, 1, 1, 1) + compactLine("2026-01-01T10:01:00.000Z", 5)
	appendFile(t, path, first)
	if _, _, err := Activity("/work/usage", "sess-u", now); err != nil {
		t.Fatal(err)
	}
	// Usage y Activity comparten pasada: ninguno relee sin cambios
	if _, err := Usage("/work/usage", "sess-u"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Activity("/work/usage", "sess-u", now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if got := usageBytesRead.Load(); got != int64(len(first)) {
		t.Errorf("leyó %d bytes, quiero %d (0 en los ciclos sin cambios)", got, len(first))
	}
}

// --- errors (DD-22) ---

func errLine(ts, id string, content string) string {
	return fmt.Sprintf(`{"type":"user","timestamp":%q,"message":{"role":"user","content":[{"type":"tool_result","tool_use_id":%q,"is_error":true,"content":%s}]}}`+"\n", ts, id, content)
}

func TestErrorsWindowAndExclusions(t *testing.T) {
	_, path := usageSetup(t)
	base := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	ts := func(min int) string {
		return base.Add(time.Duration(min) * time.Minute).Format("2006-01-02T15:04:05.000Z")
	}
	appendFile(t, path,
		errLine(ts(0), "e0", `"fallo viejo"`)+ // fuera de la ventana para now=+11
			errLine(ts(2), "e1", `"Exit code 1"`)+ // string
			errLine(ts(3), "e2", `[{"type":"text","text":"boom"}]`)+ // lista de bloques
			errLine(ts(4), "e3", `"The user doesn't want to proceed with this tool use."`)+
			errLine(ts(5), "e4", `[{"type":"text","text":"[Request interrupted by user for tool use]"}]`)+
			`{"type":"user","timestamp":"`+ts(6)+`","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"ok","content":"bien"}]}}`+"\n"+
			errLine("basura", "e5", `"sin timestamp válido"`))
	e, err := Errors("/work/usage", "sess-u", base.Add(11*time.Minute)) // ventana (+1, +11]
	if err != nil || e == nil {
		t.Fatalf("e=%v err=%v", e, err)
	}
	if e.WindowMin != 10 || e.Count != 2 || e.LastAt != base.Add(3*time.Minute).UnixMilli() {
		t.Errorf("errors=%+v, quiero 2 con lastAt=+3", e)
	}
	// ya sin nada en la ventana → nil
	if e, _ = Errors("/work/usage", "sess-u", base.Add(time.Hour)); e != nil {
		t.Errorf("fuera de ventana debe ser nil: %+v", e)
	}
	// sin mensajes con usage igual cuenta (independiente de tokens/context)
	if c, r, _ := Activity("/work/usage", "sess-u", base.Add(11*time.Minute)); c != nil || r != nil {
		t.Errorf("sin usage: context/rate deben ser nil (%v, %v)", c, r)
	}
}

func TestErrorsIncremental(t *testing.T) {
	_, path := usageSetup(t)
	now := time.Date(2026, 1, 1, 10, 5, 0, 0, time.UTC)
	first := errLine("2026-01-01T10:01:00.000Z", "e1", `"x"`) + errLine("2026-01-01T10:02:00.000Z", "e2", `"y"`)
	appendFile(t, path, first)
	e, err := Errors("/work/usage", "sess-u", now)
	if err != nil || e == nil || e.Count != 2 {
		t.Fatalf("e=%+v err=%v", e, err)
	}
	// 2º ciclo sin cambios (y otro consumidor de la misma pasada): 0 bytes nuevos
	if _, err := Errors("/work/usage", "sess-u", now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Activity("/work/usage", "sess-u", now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if got := usageBytesRead.Load(); got != int64(len(first)) {
		t.Errorf("leyó %d bytes, quiero %d (0 en los ciclos sin cambios)", got, len(first))
	}
	// una línea nueva se lee sola, sin releer lo anterior
	more := errLine("2026-01-01T10:04:00.000Z", "e3", `"z"`)
	appendFile(t, path, more)
	if e, _ = Errors("/work/usage", "sess-u", now); e == nil || e.Count != 3 {
		t.Fatalf("e=%+v, quiero 3", e)
	}
	if got := usageBytesRead.Load(); got != int64(len(first)+len(more)) {
		t.Errorf("leyó %d bytes, quiero %d", got, len(first)+len(more))
	}
}
