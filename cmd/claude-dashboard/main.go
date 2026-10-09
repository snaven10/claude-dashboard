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
	"path/filepath"
	"syscall"
	"time"

	"claude-dashboard/internal/agents"
	"claude-dashboard/internal/history"
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
	ctxWarn := flag.Int64("context-warn", 800_000, "avisa (context.warn) si el contexto actual llega a estos tokens (0 lo desactiva)")
	rateOutput := flag.Int64("rate-output", 20_000, "ritmo inusual si el output de 5 min supera esto (0 lo desactiva)")
	rateCalls := flag.Int("rate-calls", 25, "ritmo inusual si las llamadas de 5 min superan esto (0 lo desactiva)")
	errorsWarn := flag.Int("errors-warn", 5, "errors.high si los errores de herramientas de 10 min llegan a esto (0 lo desactiva)")
	histInterval := flag.Duration("history-interval", 60*time.Second, "período del escáner del historial de tokens (0 lo desactiva)")
	flag.Parse()

	if *interval <= 0 {
		return fmt.Errorf("-interval debe ser > 0")
	}
	if *stale < 0 {
		return fmt.Errorf("-stale no puede ser negativo")
	}
	if *ctxWarn < 0 || *rateOutput < 0 || *rateCalls < 0 {
		return fmt.Errorf("-context-warn, -rate-output y -rate-calls no pueden ser negativos")
	}
	if *errorsWarn < 0 {
		return fmt.Errorf("-errors-warn no puede ser negativo")
	}
	if *histInterval < 0 {
		return fmt.Errorf("-history-interval no puede ser negativo")
	}
	if err := server.ValidateAddr(*addr, *allowRemote); err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	hist := &history.Scanner{ProjectOf: agents.ProjectName}
	if *histInterval > 0 {
		if home, err := os.UserHomeDir(); err != nil {
			log.Printf("history desactivado: %v", err)
		} else {
			hist.Root = filepath.Join(home, ".claude", "projects")
			go hist.Run(ctx.Done(), *histInterval) // en segundo plano: ready:false hasta el primer escaneo
		}
	}

	s := &server.Server{
		History:     hist.Result,
		List:        agents.List,
		Summary:     transcript.Summary,
		Usage:       transcript.Usage,
		Activity:    transcript.Activity,
		Timeline:    transcript.Timeline,
		Errors:      transcript.Errors,
		Extras:      transcript.ExtrasFor,
		ErrorsWarn:  *errorsWarn,
		ContextWarn: *ctxWarn,
		RateOutput:  *rateOutput,
		RateCalls:   *rateCalls,
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
