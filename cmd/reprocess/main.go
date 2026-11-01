// Command reprocess recalcula comprobantes históricos con los módulos
// canónicos de perfiles, contratos, tasas y storage R2.
//
// Uso:
//
//	go run ./cmd/reprocess 0AFC7B97 B26D3E89 22811FF3
//	go run ./cmd/reprocess            # todas las filas sin edición manual
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"hooty/internal/clock"
	"hooty/internal/config"
	"hooty/internal/ingest/postgres"
	"hooty/internal/ingest/r2"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "reprocess:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("abrir PostgreSQL: %w", err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("conectar PostgreSQL: %w", err)
	}

	images, err := r2.New(r2.Config{
		Endpoint:     cfg.R2Endpoint,
		Bucket:       cfg.R2Bucket,
		AccessKey:    cfg.R2AccessKey,
		SecretKey:    cfg.R2SecretKey,
		PublicDomain: cfg.R2PublicDomain,
		Region:       cfg.R2Region,
		Timeout:      cfg.R2Timeout,
	})
	if err != nil {
		return err
	}
	liquidator, err := postgres.NewLiquidator(pool, clock.System{})
	if err != nil {
		return err
	}
	liquidator.WithImageResolver(images)

	hashes := os.Args[1:]
	if len(hashes) == 0 {
		count, err := liquidator.ReprocessAll(ctx)
		fmt.Printf("reprocesados=%d\n", count)
		return err
	}
	var failures []error
	count := 0
	for _, hash := range hashes {
		if _, err := liquidator.Reprocess(ctx, hash); err != nil {
			failures = append(failures, fmt.Errorf("%s: %w", hash, err))
			continue
		}
		count++
	}
	fmt.Printf("reprocesados=%d de %d\n", count, len(hashes))
	return errors.Join(failures...)
}
