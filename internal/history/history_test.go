package history

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

var utcMinus6 = time.FixedZone("UTC-6", -6*3600)

func line(ts, id, model, cwd string, in, out, cr, cc int) string {
	return fmt.Sprintf(`{"type":"assistant","timestamp":%q,"cwd":%q,"sessionId":"x","message":{"id":%q,"model":%q,"usage":{"input_tokens":%d,"output_tokens":%d,"cache_read_input_tokens":%d,"cache_creation_input_tokens":%d}}}`+"\n",
		ts, cwd, id, model, in, out, cr, cc)
}

func write(t *testing.T, path string, lines ...string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(strings.Join(lines, "")), 0o644); err != nil {
		t.Fatal(err)
	}
}

func appendTo(t *testing.T, path string, s string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := f.WriteString(s); err != nil {
		t.Fatal(err)
	}
}

func newScanner(root string) *Scanner {
	return &Scanner{Root: root, Loc: utcMinus6, Logf: func(string, ...any) {}}
}

func day(t *testing.T, r Response, date string) Day {
	t.Helper()
	for _, d := range r.Days {
		if d.Date == date {
			return d
		}
	}
	t.Fatalf("no hay día %s", date)
	return Day{}
}

var now = time.Date(2026, 10, 9, 15, 0, 0, 0, time.UTC) // 09:00 del día 9 en UTC-6

func TestDedupAndSubagentCountsParentSession(t *testing.T) {
	root := t.TempDir()
	main := filepath.Join(root, "-work-demo", "sess-A.jsonl")
	write(t, main,
		line("2026-10-09T10:00:00Z", "m1", "opus", "/work/demo", 1, 10, 100, 1000),
		line("2026-10-09T10:00:01Z", "m1", "opus", "/work/demo", 1, 10, 100, 1000), // repetido
		line("2026-10-09T10:00:02Z", "m2", "opus", "/work/demo", 2, 20, 0, 0),
	)
	sub := filepath.Join(root, "-work-demo", "sess-A", "subagents", "agent-1.jsonl")
	write(t, sub, line("2026-10-09T10:05:00Z", "s1", "haiku", "/work/demo", 0, 5, 0, 0))

	s := newScanner(root)
	if err := s.Scan(); err != nil {
		t.Fatal(err)
	}
	d := day(t, s.Result(now), "2026-10-09")
	if d.Messages != 3 || d.Input != 3 || d.Output != 35 || d.CacheRead != 100 || d.CacheCreation != 1000 {
		t.Fatalf("totales mal: %+v", d)
	}
	if d.Sessions != 1 {
		t.Fatalf("el subagente no debe ser una sesión nueva: sessions=%d", d.Sessions)
	}
}

func TestLocalDayAnd30Entries(t *testing.T) {
	root := t.TempDir()
	// 03:00Z del 9 en UTC-6 es el 8 a las 21:00
	write(t, filepath.Join(root, "p", "s1.jsonl"),
		line("2026-10-09T03:00:00Z", "a", "opus", "/w/p", 0, 7, 0, 0),
		line("2026-10-09T07:00:00Z", "b", "opus", "/w/p", 0, 3, 0, 0), // 01:00 del 9
	)
	s := newScanner(root)
	if r := s.Result(now); r.Ready || len(r.Days) != 0 {
		t.Fatalf("antes del escaneo debe ser ready:false y days vacío: %+v", r)
	}
	if err := s.Scan(); err != nil {
		t.Fatal(err)
	}
	r := s.Result(now)
	if !r.Ready || len(r.Days) != 30 {
		t.Fatalf("ready=%v días=%d", r.Ready, len(r.Days))
	}
	if r.Days[29].Date != "2026-10-09" || r.Days[0].Date != "2026-09-10" {
		t.Fatalf("extremos: %s .. %s", r.Days[0].Date, r.Days[29].Date)
	}
	for i := 1; i < len(r.Days); i++ {
		if r.Days[i-1].Date >= r.Days[i].Date {
			t.Fatalf("no ascendente en %d", i)
		}
	}
	if got := day(t, r, "2026-10-08").Output; got != 7 {
		t.Errorf("día 8 output=%d, esperaba 7", got)
	}
	if got := day(t, r, "2026-10-09").Output; got != 3 {
		t.Errorf("día 9 output=%d, esperaba 3", got)
	}
	if z := r.Days[3]; z.Output != 0 || z.Sessions != 0 || len(z.ByModel) != 0 {
		t.Errorf("día vacío debe ser ceros: %+v", z)
	}
}

func TestTop5PlusOthers(t *testing.T) {
	root := t.TempDir()
	var ls []string
	var sum int64
	for i := 1; i <= 7; i++ {
		ls = append(ls, line("2026-10-09T10:00:00Z", fmt.Sprintf("m%d", i), fmt.Sprintf("model-%d", i), fmt.Sprintf("/w/proj%d", i), 0, i*10, 0, 0))
		sum += int64(i * 10)
	}
	write(t, filepath.Join(root, "p", "s.jsonl"), ls...)
	s := newScanner(root)
	if err := s.Scan(); err != nil {
		t.Fatal(err)
	}
	d := day(t, s.Result(now), "2026-10-09")
	for name, rows := range map[string][]Row{"model": d.ByModel, "project": d.ByProject} {
		if len(rows) != 6 || rows[5].Name != "otros" {
			t.Fatalf("%s: %+v", name, rows)
		}
		if rows[0].Total != 70 || rows[4].Total != 30 || rows[5].Total != 30 { // 20+10
			t.Errorf("%s: orden/otros mal: %+v", name, rows)
		}
		var tot int64
		for _, r := range rows {
			tot += r.Total
		}
		if tot != sum {
			t.Errorf("%s: suma de filas %d != total del día %d", name, tot, sum)
		}
	}
}

func TestIncrementalTruncateDelete(t *testing.T) {
	root := t.TempDir()
	a := filepath.Join(root, "p", "a.jsonl")
	b := filepath.Join(root, "p", "b.jsonl")
	write(t, a, line("2026-10-09T10:00:00Z", "a1", "opus", "/w/p", 0, 10, 0, 0))
	write(t, b, line("2026-10-09T10:00:00Z", "b1", "opus", "/w/p", 0, 100, 0, 0))
	s := newScanner(root)
	out := func() int64 { return day(t, s.Result(now), "2026-10-09").Output }

	if err := s.Scan(); err != nil {
		t.Fatal(err)
	}
	if out() != 110 {
		t.Fatalf("inicial=%d", out())
	}
	before := s.BytesRead()
	if err := s.Scan(); err != nil {
		t.Fatal(err)
	}
	if s.BytesRead() != before {
		t.Fatalf("2º escaneo sin cambios leyó %d bytes", s.BytesRead()-before)
	}

	appendTo(t, a, line("2026-10-09T11:00:00Z", "a2", "opus", "/w/p", 0, 5, 0, 0))
	if err := s.Scan(); err != nil {
		t.Fatal(err)
	}
	if out() != 115 {
		t.Fatalf("tras append=%d", out())
	}

	// truncado: se recalcula desde 0 y NO queda lo anterior
	write(t, a, line("2026-10-09T12:00:00Z", "n1", "opus", "/w/p", 0, 1, 0, 0))
	if err := s.Scan(); err != nil {
		t.Fatal(err)
	}
	if out() != 101 {
		t.Fatalf("tras truncar=%d, esperaba 101", out())
	}

	// borrado: se resta su aporte
	if err := os.Remove(b); err != nil {
		t.Fatal(err)
	}
	if err := s.Scan(); err != nil {
		t.Fatal(err)
	}
	if out() != 1 {
		t.Fatalf("tras borrar=%d, esperaba 1", out())
	}
}

func TestChunkBoundariesAndGarbage(t *testing.T) {
	old := chunkSize
	chunkSize = 64
	defer func() { chunkSize = old }()
	root := t.TempDir()
	write(t, filepath.Join(root, "p", "s.jsonl"),
		"no es json\n",
		line("2026-10-09T10:00:00Z", "m1", "opus", "/w/p", 0, 4, 0, 0),
		`{"type":"user","message":{"content":"hola"}}`+"\n",
		line("2026-10-09T10:00:01Z", "m2", "opus", "/w/p", 0, 6, 0, 0),
	)
	s := newScanner(root)
	if err := s.Scan(); err != nil {
		t.Fatal(err)
	}
	d := day(t, s.Result(now), "2026-10-09")
	if d.Output != 10 || d.Messages != 2 {
		t.Fatalf("%+v", d)
	}
}

func TestMissingRootIsNotFatal(t *testing.T) {
	s := newScanner(filepath.Join(t.TempDir(), "nope"))
	if err := s.Scan(); err != nil {
		t.Fatal(err)
	}
	if r := s.Result(now); !r.Ready || len(r.Days) != 30 {
		t.Fatalf("%+v", r)
	}
}

// Un escaneo sin bytes nuevos no reserva el buffer de lectura: corre cada 60 s
// sobre ~1000 transcripts y antes eran 4 MB por archivo por pasada.
func TestUnchangedScanDoesNotAllocateBuffers(t *testing.T) {
	root := t.TempDir()
	for i := 0; i < 20; i++ {
		write(t, filepath.Join(root, "slug", fmt.Sprintf("s%02d.jsonl", i)),
			line("2026-10-08T12:00:00Z", fmt.Sprintf("m%d", i), "opus", "/w", 1, 2, 3, 4))
	}
	s := newScanner(root)
	if err := s.Scan(); err != nil {
		t.Fatal(err)
	}
	var a, b runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&a)
	if err := s.Scan(); err != nil {
		t.Fatal(err)
	}
	runtime.ReadMemStats(&b)
	if got := b.TotalAlloc - a.TotalAlloc; got > 1<<20 {
		t.Fatalf("2º escaneo sin cambios reservó %d bytes (quiero < 1 MB; 20 buffers de 4 MB = 80 MB)", got)
	}
}
