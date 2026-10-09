package plans

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func parse(t *testing.T, line string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "TASK-001-x.md")
	write(t, p, "# TASK\n\n"+line+"\n")
	return taskState(p)
}

func TestParseEstadoVariants(t *testing.T) {
	cases := []struct{ line, want string }{
		{"- **Estado:** `pending`", "pending"},
		{"- **Estado:** done", "done"},
		{"- **Estado:** Pendiente", "pending"},
		{"- **Estado:** pendiente", "pending"},
		{"- **Estado:** ⬜ PENDIENTE · **Depende de:** TASK-001", "pending"},
		{"- **Estado:** 🟢 `pending` — **DESBLOQUEADA, lista para correr**", "pending"},
		{"- **Estado:** `done` — cerrada 2026-08-30 (bloqueada antes)", "done"},
		{"- **Estado:** sigue `in_progress`.", "in_progress"},
		{"- **Estado:** `skipped` — DIFERIDO a PLAN-006", "skipped"},
		{"- **Estado:** `blocked`", "blocked"},
		{"- **Estado:** algo raro sin vocabulario", "unknown"},
		{"- **Estado:**", "unknown"},
		{"sin estado en el archivo", "unknown"},
	}
	for _, c := range cases {
		if got := parse(t, c.line); got != c.want {
			t.Errorf("%q → %s, want %s", c.line, got, c.want)
		}
	}
}

func TestParseOnlyFirstEstadoLine(t *testing.T) {
	// La primera línea no tiene vocabulario → unknown, aunque la segunda diga done.
	if got := parse(t, "- **Estado:** rarísimo\n- **Estado:** `done`"); got != "unknown" {
		t.Errorf("got %s", got)
	}
	if got := parse(t, "- **Estado:** `blocked`\n- **Estado:** `done`"); got != "blocked" {
		t.Errorf("got %s", got)
	}
}

func TestParseEstadoBeyondLine40Ignored(t *testing.T) {
	p := filepath.Join(t.TempDir(), "TASK-001.md")
	body := ""
	for i := 0; i < 45; i++ {
		body += "relleno\n"
	}
	write(t, p, body+"- **Estado:** `done`\n")
	if got := taskState(p); got != "unknown" {
		t.Errorf("got %s", got)
	}
}

func mkPlan(t *testing.T, base string, dirName string, states ...string) {
	t.Helper()
	for i, s := range states {
		write(t, filepath.Join(base, "plans", dirName, "tasks", fmt.Sprintf("TASK-%03d-x.md", i+1)),
			"# T\n- **Estado:** "+s+"\n")
	}
}

func TestFind(t *testing.T) {
	home := t.TempDir()

	// plan en la raíz del repo
	repoA := filepath.Join(home, "a")
	mkPlan(t, repoA, "PLAN-003-algo", "`done`", "`pending`", "`in_progress`", "`blocked`", "`skipped`", "???")
	p := Find(3, repoA, home)
	if p == nil {
		t.Fatal("plan en la raíz: nil")
	}
	want := Progress{Number: 3, Name: "PLAN-003-algo", Path: filepath.Join(repoA, "plans", "PLAN-003-algo"),
		Total: 6, Done: 1, Pending: 1, InProgress: 1, Blocked: 1, Skipped: 1, Unknown: 1}
	if *p != want {
		t.Errorf("got %+v\nwant %+v", *p, want)
	}

	// plan en un padre del repo
	parent := filepath.Join(home, "ws")
	repoB := filepath.Join(parent, "front")
	if err := os.MkdirAll(repoB, 0o755); err != nil {
		t.Fatal(err)
	}
	mkPlan(t, parent, "PLAN-042-calidad", "`done`", "`done`")
	if p := Find(133, repoB, home); p == nil || p.Total != 2 || p.Done != 2 {
		t.Errorf("plan en padre: %+v", p)
	}
	// 13 NO matchea PLAN-042
	if p := Find(13, repoB, home); p != nil {
		t.Errorf("13 matcheó PLAN-042: %+v", p)
	}
	// sin plan
	if p := Find(99, repoB, home); p != nil {
		t.Errorf("sin plan: %+v", p)
	}
	if p := Find(0, repoB, home); p != nil {
		t.Errorf("n=0: %+v", p)
	}
}

func TestFindDoesNotLeaveHome(t *testing.T) {
	outer := t.TempDir()
	home := filepath.Join(outer, "home")
	repo := filepath.Join(home, "repo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	mkPlan(t, outer, "PLAN-007-fuera", "`done`") // sobre $HOME
	if p := Find(7, repo, home); p != nil {
		t.Errorf("salió de $HOME: %+v", p)
	}
	// un repo fuera de $HOME solo mira su propia raíz
	other := filepath.Join(outer, "otro")
	if err := os.MkdirAll(other, 0o755); err != nil {
		t.Fatal(err)
	}
	if p := Find(7, other, home); p != nil {
		t.Errorf("repo fuera de $HOME subió: %+v", p)
	}
	// $HOME mismo sí se mira
	mkPlan(t, home, "PLAN-008-en-home", "`done`")
	if p := Find(8, repo, home); p == nil {
		t.Error("no miró $HOME")
	}
}

func TestEstadoChangeIsReflected(t *testing.T) {
	home := t.TempDir()
	repo := filepath.Join(home, "r")
	mkPlan(t, repo, "PLAN-001-x", "`pending`")
	if p := Find(1, repo, home); p == nil || p.Pending != 1 {
		t.Fatalf("antes: %+v", p)
	}
	f := filepath.Join(repo, "plans", "PLAN-001-x", "tasks", "TASK-001-x.md")
	write(t, f, "# T\n- **Estado:** `done` — cerrada con más texto\n")
	// mtime distinto aunque el FS tenga poca resolución
	future := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(f, future, future); err != nil {
		t.Fatal(err)
	}
	if p := Find(1, repo, home); p == nil || p.Done != 1 || p.Pending != 0 {
		t.Fatalf("después: %+v", p)
	}
}
