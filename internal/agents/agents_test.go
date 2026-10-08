package agents

import (
	"context"
	"errors"
	"os"
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
