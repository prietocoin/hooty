// Command server arranca el microservicio Hooty v2: webhook de Evolution,
// ingesta transaccional 2x y sweeper de impactos.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	hootyui "hooty"
	"hooty/internal/api"
	"hooty/internal/clock"
	"hooty/internal/config"
	"hooty/internal/ingest"
	"hooty/internal/ingest/evolution"
	"hooty/internal/ingest/gemini"
	"hooty/internal/ingest/postgres"
	"hooty/internal/ingest/r2"
	"hooty/internal/publisher"
	"hooty/internal/sweeper"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "hooty:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	logger := newLogger(cfg)
	slog.SetDefault(logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// El apagado solo se dispara por una señal real del sistema operativo.
	// No se usa signal.NotifyContext para que ningún contexto hijo cancelado
	// pueda tumbar el servidor por accidente.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(quit)

	pool, err := openPool(ctx, cfg)
	if err != nil {
		return err
	}
	defer pool.Close()

	migrateCtx, cancelMigrate := context.WithTimeout(ctx, 30*time.Second)
	err = postgres.EnsureZeroRateAllowed(migrateCtx, pool)
	if err == nil {
		err = postgres.EnsurePlantillaColumn(migrateCtx, pool)
	}
	if err == nil {
		err = postgres.EnsureTipoOpColumn(migrateCtx, pool)
	}
	if err == nil {
		err = postgres.EnsureImageURLFallback(migrateCtx, pool)
	}
	if err == nil {
		err = postgres.EnsureMaintenanceState(migrateCtx, pool)
	}
	cancelMigrate()
	if err != nil {
		return err
	}

	verifyCtx, cancelVerify := context.WithTimeout(ctx, 10*time.Second)
	err = postgres.VerifySchema(verifyCtx, pool)
	if err == nil {
		err = api.VerifyViews(verifyCtx, pool)
	}
	cancelVerify()
	if err != nil {
		return err
	}

	media, err := evolution.New(evolution.Config{
		BaseURL:  cfg.EvolutionURL,
		APIKey:   cfg.EvolutionAPIKey,
		Instance: cfg.EvolutionInst,
		Timeout:  cfg.EvolutionTimeout,
	})
	if err != nil {
		return err
	}
	extractor, err := gemini.New(gemini.Config{
		APIKeys: cfg.GeminiKeys,
		Model:   cfg.GeminiModel,
		Timeout: cfg.GeminiTimeout,
	})
	if err != nil {
		return err
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
	store, err := postgres.New(pool)
	if err != nil {
		return err
	}
	liquidator, err := postgres.NewLiquidator(pool, clock.System{})
	if err != nil {
		return err
	}
	liquidator.WithImageResolver(images)
	service, err := ingest.NewService(store, media, images, extractor, liquidator, clock.System{}, ingest.Options{
		Logger:            logger,
		ProcessingTimeout: cfg.ProcessingTimeout,
		MaxImageBytes:     cfg.MaxImageBytes,
	})
	if err != nil {
		return err
	}
	readRepository, err := api.NewRepository(ctx, pool)
	if err != nil {
		return err
	}
	sender, err := publisher.NewEvolutionSender(cfg.EvolutionURL, cfg.EvolutionAPIKey,
		cfg.EvolutionInst, cfg.EvolutionTimeout)
	if err != nil {
		return err
	}
	ratePublisher := publisher.New(readRepository, sender, publisher.NewCarteleraRenderer(),
		cfg.TestGroup, logger)
	readAPI := api.NewHandler(readRepository, logger, liquidator).WithPublisher(ratePublisher).WithNewsletterResolver(media).WithReprocessor(liquidator)

	webhook := ingest.NewHTTPHandler(service, ingest.HTTPOptions{
		Token:           cfg.WebhookToken,
		DefaultInstance: cfg.DefaultInstance,
		MaxBodyBytes:    cfg.MaxBodyBytes,
		Logger:          logger,
	})
	uiHandler, err := hootyui.UIHandler()
	if err != nil {
		return err
	}
	mux := newMux(webhook, readAPI, readyz(pool), uiHandler)

	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           api.CORS(mux),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      90 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	var retention *sweeper.Sweeper
	if cfg.SweeperEnabled {
		retention, err = sweeper.New(pool, sweeper.Options{
			Interval:           cfg.SweeperInterval,
			ProcessedRetention: cfg.ProcessedRetention,
			OrphanRetention:    cfg.OrphanRetention,
			RunOnStart:         true,
			Logger:             logger,
			Clock:              clock.System{},
		})
		if err != nil {
			return err
		}
		retention.Start(ctx)
		logger.Info("hooty.sweeper.started",
			"intervalo", cfg.SweeperInterval.String(),
			"retencion_procesados", cfg.ProcessedRetention.String(),
			"retencion_huerfanos", cfg.OrphanRetention.String(),
		)
	}

	if cfg.WebhookToken == "" {
		logger.Warn("hooty.http.sin_token: el webhook acepta cualquier cliente")
	}

	serverErr := make(chan error, 1)
	startServer := func() {
		logger.Info("hooty.http.started", "addr", cfg.HTTPAddr)
		go func() {
			err := server.ListenAndServe()
			if errors.Is(err, http.ErrServerClosed) {
				err = nil
			}
			serverErr <- err
		}()
	}
	startServer()

	// Regularización de arranque: no bloquea el HTTP. Solo toca filas no
	// editadas manualmente que quedaron sin URL o sin liquidar.
	go func() {
		reprocessCtx, cancelReprocess := context.WithTimeout(context.Background(), 15*time.Minute)
		defer cancelReprocess()
		count, err := liquidator.ReprocessHistoricalOnce(reprocessCtx)
		if err != nil {
			logger.Warn("hooty.reprocess.startup_error", "reprocesados", count, "error", err)
			return
		}
		if count > 0 {
			logger.Info("hooty.reprocess.startup", "reprocesados", count)
		}
	}()

	// El proceso permanece vivo mientras el servidor escuche. Solo una señal
	// real de parada (SIGINT/SIGTERM) o un fallo del listener terminan el run.
	for {
		select {
		case sig := <-quit:
			logger.Info("hooty.shutdown.signal", "signal", sig.String())
			cancel()

			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
			if err := server.Shutdown(shutdownCtx); err != nil {
				logger.Warn("hooty.http.shutdown_error", "error", err)
			}
			shutdownCancel()

			if retention != nil {
				retentionCtx, cancelRetention := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
				if err := retention.Close(retentionCtx); err != nil {
					logger.Warn("hooty.sweeper.shutdown_error", "error", err)
				}
				cancelRetention()
			}
			logger.Info("hooty.shutdown.done")
			return nil
		case err := <-serverErr:
			if err != nil {
				return fmt.Errorf("servidor HTTP: %w", err)
			}
			// El listener se cerró sin una señal de parada: no se debe apagar
			// el proceso; se vuelve a levantar el servidor.
			logger.Warn("hooty.http.server_exited", "addr", cfg.HTTPAddr)
			time.Sleep(time.Second)
			startServer()
		}
	}
}

// newMux ensambla las rutas del servidor. Se aísla para que los tests puedan
// verificar precedencia y barra final sin abrir PostgreSQL.
func newMux(webhook http.Handler, readAPI *api.Handler, ready http.Handler, ui http.Handler) *http.ServeMux {
	mux := http.NewServeMux()
	mux.Handle("POST /webhooks/evolution", webhook)
	mux.Handle("POST /api/v1/webhooks/hooty", webhook)
	readAPI.Register(mux)
	// /healthz es liveness puro: mientras el proceso escuche, el orquestador no
	// debe reiniciarlo por una caída transitoria de PostgreSQL. La dependencia
	// de base de datos queda en /readyz.
	mux.HandleFunc("GET /healthz", livez)
	mux.Handle("GET /readyz", ready)
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/comprobantes/", http.StatusPermanentRedirect)
	})
	mux.Handle("GET /", ui)
	return mux
}

func openPool(ctx context.Context, cfg config.Config) (*pgxpool.Pool, error) {
	poolCfg, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("parsear DATABASE_URL: %w", err)
	}
	poolCfg.MaxConns = cfg.DBMaxConns
	poolCfg.MinConns = 1
	poolCfg.MaxConnLifetime = time.Hour
	poolCfg.MaxConnIdleTime = 30 * time.Minute
	poolCfg.ConnConfig.ConnectTimeout = 10 * time.Second
	if poolCfg.ConnConfig.RuntimeParams == nil {
		poolCfg.ConnConfig.RuntimeParams = map[string]string{}
	}
	// El sellado de hora es explícito en cada escritura, pero fijar la sesión
	// evita que un NOW() accidental use UTC.
	poolCfg.ConnConfig.RuntimeParams["timezone"] = "America/Caracas"

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("crear pool PostgreSQL: %w", err)
	}
	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("conectar PostgreSQL: %w", err)
	}
	return pool, nil
}

func livez(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "method_not_allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

func readyz(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			http.Error(w, "method_not_allowed", http.StatusMethodNotAllowed)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := pool.Ping(ctx); err != nil {
			http.Error(w, "database_unavailable", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}
}

func newLogger(cfg config.Config) *slog.Logger {
	level := slog.LevelInfo
	switch cfg.LogLevel {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}
	opts := &slog.HandlerOptions{Level: level}
	if cfg.LogFormat == "json" {
		return slog.New(slog.NewJSONHandler(os.Stdout, opts))
	}
	return slog.New(slog.NewTextHandler(os.Stdout, opts))
}
