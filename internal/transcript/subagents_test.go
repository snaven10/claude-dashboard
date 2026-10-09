package transcript

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSubagentsActiveWindow(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	mk := func(name string, age time.Duration) {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte("{}\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(p, now.Add(-age), now.Add(-age)); err != nil {
			t.Fatal(err)
		}
	}
	mk("a.jsonl", 10*time.Second)  // activo
	mk("b.jsonl", 119*time.Second) // activo
	mk("c.jsonl", 121*time.Second) // inactivo
	mk("d.jsonl", time.Hour)       // inactivo
	mk("notes.txt", time.Second)   // no es jsonl
	if err := os.Mkdir(filepath.Join(dir, "x.jsonl.d"), 0o755); err != nil {
		t.Fatal(err)
	}
	got := Subagents(dir, now)
	if got == nil || got.Active != 2 || got.Total != 4 {
		t.Fatalf("subagents = %+v, quiero active 2 total 4", got)
	}
}

func TestSubagentsNoDirIsNil(t *testing.T) {
	if got := Subagents(filepath.Join(t.TempDir(), "no-existe"), time.Now()); got != nil {
		t.Errorf("sin directorio debe ser nil: %+v", got)
	}
}

func TestExtrasForIncludesSubagents(t *testing.T) {
	_, path := usageSetup(t)
	if err := os.WriteFile(path, []byte(userAt("2026-01-01T10:00:00.000Z", "hola")), 0o600); err != nil {
		t.Fatal(err)
	}
	e, err := ExtrasFor("/work/usage", "sess-u", time.Now())
	if err != nil || e == nil || e.Subagents != nil {
		t.Fatalf("sin directorio subagents debe ser nil: %+v err=%v", e, err)
	}
	sub := filepath.Join(filepath.Dir(path), "sess-u", "subagents")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "agent-1.jsonl"), []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	e, _ = ExtrasFor("/work/usage", "sess-u", time.Now())
	if e.Subagents == nil || e.Subagents.Active != 1 || e.Subagents.Total != 1 {
		t.Errorf("subagents = %+v", e.Subagents)
	}
}
