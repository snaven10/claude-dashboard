package remote

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sync"
)

const (
	dirMode  os.FileMode = 0o700
	fileMode os.FileMode = 0o600
)

// Dir resuelve y crea el directorio de estado (DD-45) con 0700. stateDir vacío
// = os.UserConfigDir()/claude-dashboard. Nunca apunta a ~/.claude.
func Dir(stateDir string) (string, error) {
	if stateDir == "" {
		base, err := os.UserConfigDir()
		if err != nil {
			return "", fmt.Errorf("resolviendo el directorio de configuración: %w", err)
		}
		stateDir = filepath.Join(base, "claude-dashboard")
	}
	if err := os.MkdirAll(stateDir, dirMode); err != nil {
		return "", fmt.Errorf("creando %s: %w", stateDir, err)
	}
	if err := os.Chmod(stateDir, dirMode); err != nil {
		return "", fmt.Errorf("permisos de %s: %w", stateDir, err)
	}
	return stateDir, nil
}

// Store lee y escribe archivos JSON dentro del directorio de estado.
// Un mutex por store serializa lecturas y escrituras.
type Store struct {
	dir string
	mu  sync.Mutex
}

// NewStore abre (y crea si falta) el directorio de estado.
func NewStore(stateDir string) (*Store, error) {
	dir, err := Dir(stateDir)
	if err != nil {
		return nil, err
	}
	return &Store{dir: dir}, nil
}

// Path devuelve la ruta de un archivo del store.
func (s *Store) Path(name string) string { return filepath.Join(s.dir, name) }

// Read decodifica name en v; archivo inexistente = v intacto, sin error.
func (s *Store) Read(name string, v any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return readJSON(s.Path(name), v)
}

// Write persiste v en name de forma atómica con 0600.
func (s *Store) Write(name string, v any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return writeJSON(s.Path(name), v)
}

// readJSON: inexistente = sin error y v intacto. Si el modo es más abierto que
// 0600 se corrige y se loguea (DD-45), sin abortar.
func readJSON(path string, v any) error {
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("abriendo %s: %w", path, err)
	}
	defer f.Close()
	if st, err := f.Stat(); err == nil && st.Mode().Perm()&^fileMode != 0 {
		log.Printf("remote: %s tenía permisos %v; corrigiendo a %v", path, st.Mode().Perm(), fileMode)
		if err := os.Chmod(path, fileMode); err != nil {
			return fmt.Errorf("corrigiendo permisos de %s: %w", path, err)
		}
	}
	if err := json.NewDecoder(f).Decode(v); err != nil {
		return fmt.Errorf("decodificando %s: %w", path, err)
	}
	return nil
}

// writeJSON: temp en el mismo dir → Write → Sync → Chmod 0600 → Rename.
func writeJSON(path string, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("codificando %s: %w", path, err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("creando temporal para %s: %w", path, err)
	}
	tmpName := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpName) }
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		cleanup()
		return fmt.Errorf("escribiendo %s: %w", tmpName, err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		cleanup()
		return fmt.Errorf("fsync %s: %w", tmpName, err)
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return fmt.Errorf("cerrando %s: %w", tmpName, err)
	}
	if err := os.Chmod(tmpName, fileMode); err != nil {
		cleanup()
		return fmt.Errorf("chmod %s: %w", tmpName, err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		cleanup()
		return fmt.Errorf("renombrando a %s: %w", path, err)
	}
	return nil
}
