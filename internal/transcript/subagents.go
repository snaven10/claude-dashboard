package transcript

import (
	"os"
	"strings"
	"time"
)

// subagentActiveWindow: un subagente cuenta como activo si su transcript se
// modificó dentro de esta ventana respecto de `now` (DD-35).
const subagentActiveWindow = 120 * time.Second

// SubagentStats cuenta los transcripts de subagentes de una sesión.
type SubagentStats struct {
	Active int `json:"active"`
	Total  int `json:"total"`
}

// Subagents cuenta los *.jsonl de dir. Un solo ReadDir y un Info por archivo,
// sin leer contenido. nil si el directorio no existe o no se puede leer.
func Subagents(dir string, now time.Time) *SubagentStats {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	s := &SubagentStats{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}
		info, err := e.Info()
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		s.Total++
		if now.Sub(info.ModTime()) <= subagentActiveWindow {
			s.Active++
		}
	}
	return s
}
