// Command claude-dashboard es el daemon local que muestra las sesiones de
// Claude Code en http://localhost:7777.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"claude-dashboard/internal/agents"
	"claude-dashboard/internal/orq"
	"claude-dashboard/internal/server"
	"claude-dashboard/internal/transcript"
	"claude-dashboard/web"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "claude-dashboard:", err)
		os.Exit(1)
	}
}

func run() error {
	addr := flag.String("addr", "127.0.0.1:7777", "dirección de escucha (solo loopback salvo -allow-remote)")
	interval := flag.Duration("interval", 3*time.Second, "período del poller")
	allowRemote := flag.Bool("allow-remote", false, "permitir un -addr que no sea loopback (expone prompts)")
	stale := flag.Duration("stale", 10*time.Minute, "una sesión busy sin actividad por más de esto se marca stale (0 la desactiva)")
	flag.Parse()

	if *interval <= 0 {
		return fmt.Errorf("-interval debe ser > 0")
	}
	if *stale < 0 {
		return fmt.Errorf("-stale no puede ser negativo")
	}
	if err := server.ValidateAddr(*addr, *allowRemote); err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	s := &server.Server{
		List:     agents.List,
		Summary:  transcript.Summary,
		Usage:    transcript.Usage,
		Timeline: transcript.Timeline,
		Orq: func() (map[string]orq.Job, error) {
			path, err := orq.DefaultPath()
			if err != nil {
				return nil, err
			}
			return orq.Load(path)
		},
		StaleAfter: *stale,
		Static:     web.FS,
		Interval:   *interval,
		Logf:       log.Printf,
	}
	return s.ListenAndServe(ctx, *addr)
}
