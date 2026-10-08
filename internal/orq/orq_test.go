package orq

import (
	"os"
	"path/filepath"
	"testing"
)

func write(t *testing.T, dir, content string) string {
	t.Helper()
	p := filepath.Join(dir, "jobs.json")
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLoadIndexesByBgnameAndPlanTask(t *testing.T) {
	p := write(t, t.TempDir(), `{
	  "9efa2c1a": {"key":"claude-dashboard/p2-task006","ws":"claude-dashboard","specialist":"claude-dashboard-worker","bgname":"w-006"},
	  "aaaa0000": {"key":"manual","ws":"x","specialist":"s","bgname":"w-manual"},
	  "bbbb1111": {"key":"sin-bgname","ws":"x","specialist":"s"}
	}`)
	jobs, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 2 {
		t.Fatalf("jobs=%d, quiero 2 (sin bgname se ignora)", len(jobs))
	}
	j := jobs["w-006"]
	if j.ID != "9efa2c1a" || j.WS != "claude-dashboard" || j.Specialist != "claude-dashboard-worker" {
		t.Errorf("job: %+v", j)
	}
	if plan, task, ok := j.PlanTask(); !ok || plan != "2" || task != "006" {
		t.Errorf("plan/task = %q %q %v", plan, task, ok)
	}
	if _, _, ok := jobs["w-manual"].PlanTask(); ok {
		t.Error("key sin patrón no debe matchear")
	}
}

func TestLoadMissingAndBroken(t *testing.T) {
	dir := t.TempDir()
	jobs, err := Load(filepath.Join(dir, "no-existe.json"))
	if err != nil || len(jobs) != 0 {
		t.Fatalf("ausente: %v %v", jobs, err)
	}
	p := write(t, dir, `{esto no es json`)
	jobs, err = Load(p)
	if err == nil || jobs == nil || len(jobs) != 0 {
		t.Fatalf("roto: jobs=%v err=%v (quiero mapa vacío no-nil y error)", jobs, err)
	}
	// segunda lectura de la misma versión sale del cache, sin error y vacío
	jobs, err = Load(p)
	if err != nil || len(jobs) != 0 {
		t.Fatalf("roto cacheado: %v %v", jobs, err)
	}
}

func TestLoadCacheByMtime(t *testing.T) {
	dir := t.TempDir()
	p := write(t, dir, `{"a":{"key":"p1-task001","bgname":"x"}}`)
	a, _ := Load(p)
	b, _ := Load(p)
	if len(a) != 1 || len(b) != 1 {
		t.Fatal("debería traer 1 job")
	}
	// cambia contenido (y tamaño): se vuelve a leer
	write(t, dir, `{"a":{"key":"p1-task001","bgname":"x"},"b":{"key":"p1-task002","bgname":"y"}}`)
	c, _ := Load(p)
	if len(c) != 2 {
		t.Fatalf("no recargó tras el cambio: %d", len(c))
	}
}
