package transcript

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

// Tokens son los totales de uso de una sesión (DD-9). Sin costo en dólares (DD-11).
type Tokens struct {
	Input         int64  `json:"input"`
	Output        int64  `json:"output"`
	CacheRead     int64  `json:"cacheRead"`
	CacheCreation int64  `json:"cacheCreation"`
	Model         string `json:"model"`
}

// usageBytesRead cuenta los bytes leídos por el acumulador (para tests).
var usageBytesRead atomic.Int64

// usageState es el acumulador de un archivo. Se lee de offset al final.
type usageState struct {
	mu            sync.Mutex
	offset        int64
	totals        Tokens
	lastMessageID string
	counted       bool // hubo al menos un mensaje con usage
}

var (
	usageMu     sync.Mutex
	usageStates = map[string]*usageState{}
)

func stateFor(path string) *usageState {
	usageMu.Lock()
	defer usageMu.Unlock()
	st, ok := usageStates[path]
	if !ok {
		st = &usageState{}
		usageStates[path] = st
	}
	return st
}

type usageLine struct {
	Type    string `json:"type"`
	Message struct {
		ID    string `json:"id"`
		Model string `json:"model"`
		Usage *struct {
			Input         int64 `json:"input_tokens"`
			Output        int64 `json:"output_tokens"`
			CacheRead     int64 `json:"cache_read_input_tokens"`
			CacheCreation int64 `json:"cache_creation_input_tokens"`
		} `json:"usage"`
	} `json:"message"`
}

// Usage devuelve los tokens acumulados de la sesión. nil si no hay transcript
// o todavía no hay mensajes del asistente con usage. Lee solo lo nuevo desde el
// último ciclo; la primera lectura de un transcript grande es entera y se loguea.
func Usage(cwd, sessionID string) (*Tokens, error) {
	root, err := projectsRoot()
	if err != nil {
		return nil, err
	}
	path := findPath(root, cwd, sessionID)
	if path == "" {
		return nil, nil
	}
	t, err := accumulate(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("tokens de %s: %w", path, err)
	}
	return t, nil
}

func accumulate(path string) (*Tokens, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}
	size := fi.Size()

	st := stateFor(path)
	st.mu.Lock()
	defer st.mu.Unlock()

	if size < st.offset { // truncado o rotado
		st.offset, st.totals, st.lastMessageID, st.counted = 0, Tokens{}, "", false
	}
	if size > st.offset {
		first := st.offset == 0
		start := time.Now()
		buf := make([]byte, size-st.offset)
		n, err := f.ReadAt(buf, st.offset)
		if err != nil && !errors.Is(err, io.EOF) {
			return nil, err
		}
		usageBytesRead.Add(int64(n))
		buf = buf[:n]
		// solo líneas completas: lo que sigue al último '\n' queda para el próximo ciclo
		end := bytes.LastIndexByte(buf, '\n')
		if end >= 0 {
			st.consume(buf[:end+1])
			st.offset += int64(end + 1)
		}
		if first && n > initialWindow {
			log.Printf("transcript: primera lectura completa de %s: %d bytes en %s", path, n, time.Since(start).Round(time.Millisecond))
		}
	}
	if !st.counted {
		return nil, nil
	}
	t := st.totals
	return &t, nil
}

func (st *usageState) consume(data []byte) {
	for len(data) > 0 {
		i := bytes.IndexByte(data, '\n')
		raw := data[:i]
		data = data[i+1:]
		raw = bytes.TrimSpace(raw)
		if len(raw) == 0 {
			continue
		}
		var l usageLine
		if json.Unmarshal(raw, &l) != nil || l.Type != "assistant" || l.Message.Usage == nil {
			continue
		}
		if m := l.Message.Model; m != "" {
			st.totals.Model = m
		}
		id := l.Message.ID
		if id != "" && id == st.lastMessageID { // línea repetida del mismo mensaje
			continue
		}
		u := l.Message.Usage
		st.totals.Input += u.Input
		st.totals.Output += u.Output
		st.totals.CacheRead += u.CacheRead
		st.totals.CacheCreation += u.CacheCreation
		st.lastMessageID = id
		st.counted = true
	}
}
