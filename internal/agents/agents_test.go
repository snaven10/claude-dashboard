package agents

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fixtureRunner(t *testing.T) Runner {
	t.Helper()
	data, err := os.ReadFile("testdata/f1.json")
	if err != nil {
		t.Fatal(err)
	}
	return func(context.Context) ([]byte, error) { return data, nil }
}

func listByName(t *testing.T) map[string]Session {
	t.Helper()
	c := Collector{Run: fixtureRunner(t), SessionsDir: "testdata/sessions"}
	got, err := c.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 7 {
		t.Fatalf("esperaba 7 sesiones, hay %d", len(got))
	}
	m := map[string]Session{}
	for _, s := range got {
		m[s.Name] = s
	}
	return m
}

func TestListNormalizacion(t *testing.T) {
	byName := listByName(t)
	want := map[string]string{
		"a": StatusBusy, "b": StatusIdle, "c": StatusWaiting,
		"d": StatusBlocked, "e": StatusBusy, "f": StatusDone,
		// Forma real de una background terminada: state manda sobre status.
		"g": StatusDone,
	}
	for n, st := range want {
		if byName[n].Status != st {
			t.Errorf("sesión %s: status %q, esperaba %q", n, byName[n].Status, st)
		}
	}
	if w := byName["c"].WaitingFor; w == nil || *w != "input needed" {
		t.Errorf("waitingFor de c = %v", w)
	}
	if byName["b"].Project != "proj-b" {
		t.Errorf("project de b = %q", byName["b"].Project)
	}
	if byName["b"].ID != "bbbb2222" {
		t.Errorf("id derivado de b = %q", byName["b"].ID)
	}
}

func TestEnriquecimientoF2(t *testing.T) {
	byName := listByName(t)
	if a := byName["a"]; a.LastActivityAt != 1234567 || !a.Remote {
		t.Errorf("a no enriquecida: %+v", a)
	}
	// 102.json es basura y 103.json no existe: degradan sin error.
	if b := byName["b"]; b.LastActivityAt != 2000 || b.Remote {
		t.Errorf("b debía degradar a startedAt: %+v", b)
	}
	if c := byName["c"]; c.LastActivityAt != 3000 {
		t.Errorf("c debía degradar: %+v", c)
	}
}

func TestRunnerFalla(t *testing.T) {
	boom := errors.New("boom")
	c := Collector{Run: func(context.Context) ([]byte, error) { return nil, boom }}
	if _, err := c.List(context.Background()); !errors.Is(err, boom) {
		t.Fatalf("esperaba boom, got %v", err)
	}
}

func TestJSONInvalido(t *testing.T) {
	c := Collector{Run: func(context.Context) ([]byte, error) { return []byte("nope"), nil }}
	if _, err := c.List(context.Background()); err == nil {
		t.Fatal("esperaba error de parseo")
	}
}

func TestClaudeNoEnPATH(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	_, err := List(context.Background())
	if err == nil || !strings.Contains(err.Error(), "claude") {
		t.Fatalf("esperaba error claro sobre claude, got %v", err)
	}
}

func TestProjectName(t *testing.T) {
	root := t.TempDir()
	mk := func(p string) string {
		d := filepath.Join(root, p)
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
		return d
	}
	// Repo principal: .git es un directorio.
	repo := mk("repos/mi-repo")
	mk("repos/mi-repo/.git")
	sub := mk("repos/mi-repo/internal/x")
	// Worktree: .git es un archivo apuntando a <repo>/.git/worktrees/<n>.
	wt := mk("trees/ws/mi-repo/plan-1-task-9")
	gitdir := "gitdir: " + filepath.Join(repo, ".git", "worktrees", "plan-1-task-9") + "\n"
	if err := os.WriteFile(filepath.Join(wt, ".git"), []byte(gitdir), 0o644); err != nil {
		t.Fatal(err)
	}
	wtSub := mk("trees/ws/mi-repo/plan-1-task-9/cmd")
	// .git archivo ilegible/ajeno → basename(cwd).
	odd := mk("odd/carpeta")
	if err := os.WriteFile(filepath.Join(odd, ".git"), []byte("basura"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Sin repo (el TempDir no está dentro de ningún .git).
	plain := mk("sin/repo")

	cases := map[string]string{
		repo: "mi-repo", sub: "mi-repo", wt: "mi-repo", wtSub: "mi-repo",
		odd: "carpeta", plain: "repo", "": "",
	}
	for cwd, want := range cases {
		if got := projectName(cwd); got != want {
			t.Errorf("projectName(%q) = %q, esperaba %q", cwd, got, want)
		}
	}
}

func TestRemoteURL(t *testing.T) {
	valid := "session_01FakeFixtureIdAbCdEfGhI"
	got := remoteURL(valid)
	if got == nil || *got != "https://claude.ai/code/"+valid {
		t.Fatalf("id válido: %v", got)
	}
	invalid := map[string]string{
		"vacío":         "",
		"formato viejo": "bridge-x",
		"con /":         "session_01FakeFi/../x",
		"con ?":         "session_01FakeFixtureI?x=1",
		"con espacios":  "session_01FakeFi bAAutv",
		"con salto":     "session_01FakeFixtureI\n",
		"muy corto":     "session_abc",
		"muy largo":     "session_" + strings.Repeat("a", 65),
		"sin prefijo":   "01FakeFixtureIdAbCdEfGhI",
		"con guion":     "session_01FakeFi-bAAutv",
	}
	for name, id := range invalid {
		if got := remoteURL(id); got != nil {
			t.Errorf("%s (%q): esperaba nil, got %q", name, id, *got)
		}
	}
	if long := remoteURL("session_" + strings.Repeat("a", 64)); long == nil {
		t.Error("64 caracteres es el máximo válido")
	}
}

func TestRemoteURLEnNormalize(t *testing.T) {
	id := "session_01FakeFixtureIdAbCdEfGhI"
	s := normalize(rawAgent{ID: "x", Kind: "interactive"}, rawSession{BridgeSessionID: id})
	if !s.Remote || s.RemoteURL == nil || *s.RemoteURL != "https://claude.ai/code/"+id {
		t.Errorf("válido: %+v", s)
	}
	// id raro: sigue "remote" como hoy, pero sin URL.
	s = normalize(rawAgent{ID: "x"}, rawSession{BridgeSessionID: "bridge-x"})
	if !s.Remote || s.RemoteURL != nil {
		t.Errorf("inválido: %+v", s)
	}
	if s = normalize(rawAgent{ID: "x"}, rawSession{}); s.Remote || s.RemoteURL != nil {
		t.Errorf("sin bridge: %+v", s)
	}
}
