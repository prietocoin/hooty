// Package sweeper purga impactos antiguos en segundo plano.
//
// Reglas:
//   - ticker de una hora (configurable);
//   - pg_try_advisory_lock para que solo una réplica ejecute la pasada;
//   - procesados con más de una hora y huérfanos con más de 24 horas;
//   - jamás toca la tabla comprobantes.
package sweeper

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"hooty/internal/clock"
)

// advisoryLockKey es "HOOTYSW" en hexadecimal. El lock es de sesión y vive en
// una conexión dedicada durante toda la pasada.
const advisoryLockKey int64 = 0x484F4F54595357

// Result resume una pasada.
type Result struct {
	Procesados int64
	Huerfanos  int64
	Pares      int64
	Duration   time.Duration
}

// Options configura retenciones y ciclo de vida.
type Options struct {
	Interval           time.Duration
	ProcessedRetention time.Duration
	OrphanRetention    time.Duration
	RunOnStart         bool
	Logger             *slog.Logger
	Clock              clock.Clock
}

// Sweeper es el worker de retención.
type Sweeper struct {
	pool *pgxpool.Pool
	opts Options

	mu      sync.Mutex
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	running bool
}

// New valida la configuración. No abre conexiones ni arranca el ticker.
func New(pool *pgxpool.Pool, opts Options) (*Sweeper, error) {
	if pool == nil {
		return nil, errors.New("sweeper: pool es obligatorio")
	}
	if err := validateOptions(opts); err != nil {
		return nil, err
	}
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	if opts.Clock == nil {
		opts.Clock = clock.System{}
	}
	return &Sweeper{pool: pool, opts: opts}, nil
}

func validateOptions(opts Options) error {
	if opts.Interval <= 0 {
		return errors.New("sweeper: intervalo debe ser positivo")
	}
	if opts.ProcessedRetention <= 0 {
		return errors.New("sweeper: retención de procesados debe ser positiva")
	}
	if opts.OrphanRetention <= 0 {
		return errors.New("sweeper: retención de huérfanos debe ser positiva")
	}
	return nil
}

// Start lanza el worker. Es idempotente y no bloquea.
func (s *Sweeper) Start(ctx context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running {
		return
	}
	runCtx, cancel := context.WithCancel(ctx)
	s.cancel = cancel
	s.running = true
	s.wg.Add(1)
	go s.loop(runCtx)
}

// Close cancela el ticker y espera a que termine la pasada en curso.
func (s *Sweeper) Close(ctx context.Context) error {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return nil
	}
	s.running = false
	cancel := s.cancel
	s.cancel = nil
	s.mu.Unlock()

	cancel()
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-done:
		return nil
	}
}

func (s *Sweeper) loop(ctx context.Context) {
	defer s.wg.Done()
	if s.opts.RunOnStart {
		s.runAndLog(ctx)
	}

	ticker := time.NewTicker(s.opts.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.runAndLog(ctx)
		}
	}
}

func (s *Sweeper) runAndLog(ctx context.Context) {
	result, err := s.RunOnce(ctx)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return
		}
		s.opts.Logger.Warn("hooty.sweeper.error", "error", err)
		return
	}
	if result.Procesados == 0 && result.Huerfanos == 0 && result.Pares == 0 && result.Duration == 0 {
		return
	}
	s.opts.Logger.Info("hooty.sweeper",
		"procesados", result.Procesados,
		"huerfanos", result.Huerfanos,
		"pares", result.Pares,
		"duracion_ms", result.Duration.Milliseconds(),
	)
}

// RunOnce ejecuta una pasada. Si otra réplica tiene el advisory lock, devuelve
// Result{} sin error: no es un fallo, es contención normal.
func (s *Sweeper) RunOnce(ctx context.Context) (Result, error) {
	started := time.Now()
	conn, err := s.pool.Acquire(ctx)
	if err != nil {
		return Result{}, fmt.Errorf("sweeper: adquirir conexión: %w", err)
	}
	defer conn.Release()

	var locked bool
	if err := conn.QueryRow(ctx, sqlTryLock, advisoryLockKey).Scan(&locked); err != nil {
		return Result{}, fmt.Errorf("sweeper: advisory lock: %w", err)
	}
	if !locked {
		s.opts.Logger.Debug("hooty.sweeper.lock_ocupado")
		return Result{}, nil
	}
	defer func() {
		unlockCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, err := conn.Exec(unlockCtx, sqlUnlock, advisoryLockKey); err != nil {
			s.opts.Logger.Warn("hooty.sweeper.unlock_error", "error", err)
		}
	}()

	now := s.opts.Clock.Now()
	if now.IsZero() {
		return Result{}, errors.New("sweeper: el reloj devolvió una hora cero")
	}
	processedCutoff := clock.ForTimestamp(now.Add(-s.opts.ProcessedRetention))
	orphanCutoff := clock.ForTimestamp(now.Add(-s.opts.OrphanRetention))

	tx, err := conn.Begin(ctx)
	if err != nil {
		return Result{}, fmt.Errorf("sweeper: abrir transacción: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	processed, err := tx.Exec(ctx, sqlDeleteProcessed, processedCutoff)
	if err != nil {
		return Result{}, fmt.Errorf("sweeper: borrar procesados: %w", err)
	}
	orphans, err := tx.Exec(ctx, sqlDeleteOrphans, orphanCutoff)
	if err != nil {
		return Result{}, fmt.Errorf("sweeper: borrar huérfanos: %w", err)
	}
	pairs, err := tx.Exec(ctx, sqlDeleteFailedPairs, orphanCutoff)
	if err != nil {
		return Result{}, fmt.Errorf("sweeper: borrar pares fallidos: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Result{}, fmt.Errorf("sweeper: commit: %w", err)
	}

	return Result{
		Procesados: processed.RowsAffected(),
		Huerfanos:  orphans.RowsAffected(),
		Pares:      pairs.RowsAffected(),
		Duration:   time.Since(started),
	}, nil
}
