// Package orq lee ~/.orq/jobs.json para saber qué sesiones de F1 son workers
// de orq (DD-10). Es enriquecimiento: si el archivo falta o está roto, el
// resultado es un mapa vacío y el panel sigue funcionando.
package orq

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"time"
)

// Job es un job de orq tal como el panel lo necesita.
type Job struct {
	ID         string // jid (clave en jobs.json)
	Key        string
	WS         string
	Specialist string
	Bgname     string
}

type rawJob struct {
	Key        string `json:"key"`
	WS         string `json:"ws"`
	Specialist string `json:"specialist"`
	Bgname     string `json:"bgname"`
}

var keyRe = regexp.MustCompile(`p(\d+)-task(\d+)`)

// PlanTask saca plan y task del key del job ("…p2-task006…"). ok=false si no matchea.
func (j Job) PlanTask() (plan, task string, ok bool) {
	m := keyRe.FindStringSubmatch(j.Key)
	if m == nil {
		return "", "", false
	}
	return m[1], m[2], true
}

// DefaultPath devuelve ~/.orq/jobs.json.
func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolviendo home: %w", err)
	}
	return filepath.Join(home, ".orq", "jobs.json"), nil
}

type cacheEntry struct {
	mtime time.Time
	size  int64
	jobs  map[string]Job
}

var (
	mu    sync.Mutex
	cache = map[string]cacheEntry{}
)

// Load devuelve los jobs indexados por bgname. Cachea por (mtime, size).
// Archivo ausente → mapa vacío y nil. Archivo roto → mapa vacío y error (solo
// la primera vez que se lee esa versión; después sale del cache sin error).
func Load(path string) (map[string]Job, error) {
	st, err := os.Stat(path)
	if err != nil {
		mu.Lock()
		delete(cache, path)
		mu.Unlock()
		if errors.Is(err, fs.ErrNotExist) {
			return map[string]Job{}, nil
		}
		return map[string]Job{}, fmt.Errorf("stat %s: %w", path, err)
	}
	mu.Lock()
	if e, ok := cache[path]; ok && e.mtime.Equal(st.ModTime()) && e.size == st.Size() {
		mu.Unlock()
		return e.jobs, nil
	}
	mu.Unlock()

	jobs, perr := parseFile(path)
	mu.Lock()
	cache[path] = cacheEntry{mtime: st.ModTime(), size: st.Size(), jobs: jobs}
	mu.Unlock()
	return jobs, perr
}

func parseFile(path string) (map[string]Job, error) {
	out := map[string]Job{}
	data, err := os.ReadFile(path)
	if err != nil {
		return out, fmt.Errorf("leyendo %s: %w", path, err)
	}
	var raw map[string]rawJob
	if err := json.Unmarshal(data, &raw); err != nil {
		return out, fmt.Errorf("parseando %s: %w", path, err)
	}
	for id, r := range raw {
		if r.Bgname == "" {
			continue
		}
		out[r.Bgname] = Job{ID: id, Key: r.Key, WS: r.WS, Specialist: r.Specialist, Bgname: r.Bgname}
	}
	return out, nil
}
