// Copyright 2026 [Copyright Holder]
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// Author: [YOUR_NAME]

package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/pprof"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/sh0jitmy/pgstate-gateway/internal/api"
	"github.com/sh0jitmy/pgstate-gateway/internal/config"
	"github.com/sh0jitmy/pgstate-gateway/internal/logger"
	"github.com/sh0jitmy/pgstate-gateway/internal/metrics"
	"github.com/sh0jitmy/pgstate-gateway/internal/repository/postgres"
	"github.com/sh0jitmy/pgstate-gateway/internal/service"
	"github.com/sh0jitmy/pgstate-gateway/migrations"
	"github.com/urfave/cli/v2"
	"go.uber.org/zap"
	"golang.org/x/crypto/acme/autocert"
)

// Version represents the application version, typically injected via build ldflags.
var Version = "0.0.0-dev"

func main() {
	app := &cli.App{
		Name:    "tf-http-backend",
		Usage:   "Production-ready Terraform HTTP Backend Server backed by PostgreSQL",
		Version: Version,
		Commands: []*cli.Command{
			{
				Name:  "serve",
				Usage: "Start the HTTP/HTTPS backend server",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "config",
						Aliases: []string{"c"},
						Usage:   "Path to configuration YAML file",
					},
				},
				Action: func(c *cli.Context) error {
					return runServer(c.Context, c.String("config"))
				},
			},
			{
				Name:  "migrate",
				Usage: "Database migration management",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "config",
						Aliases: []string{"c"},
						Usage:   "Path to configuration YAML file",
					},
				},
				Subcommands: []*cli.Command{
					{
						Name:  "up",
						Usage: "Apply up migrations",
						Action: func(c *cli.Context) error {
							return runMigrate(c.String("config"), "up")
						},
					},
					{
						Name:  "down",
						Usage: "Apply down migrations",
						Action: func(c *cli.Context) error {
							return runMigrate(c.String("config"), "down")
						},
					},
				},
			},
			{
				Name:  "version",
				Usage: "Print the application version",
				Action: func(c *cli.Context) error {
					fmt.Printf("tf-http-backend version %s\n", Version)
					return nil
				},
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: %v\n", err)
		os.Exit(1)
	}
}

func startSecurePprof() *http.Server {
	// Enable mutex/block profiling as requested in golang-observability:4.2
	runtime.SetMutexProfileFraction(10)
	runtime.SetBlockProfileRate(10000)

	mux := http.NewServeMux()
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)

	srv := &http.Server{
		Addr:              "127.0.0.1:6060",
		Handler:           mux,
		ReadHeaderTimeout: 3 * time.Second,
	}

	go func() {
		logger.Log.Info("Starting secure local pprof server...", zap.String("address", srv.Addr))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Log.Error("pprof server execution failed", zap.Error(err))
		}
	}()

	return srv
}

func runServer(ctx context.Context, configPath string) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if logInitErr := logger.Init(cfg.Logging.Level); logInitErr != nil {
		return fmt.Errorf("failed to initialize logger: %w", logInitErr)
	}

	configMgr := config.NewManager(cfg)

	// Start secure localhost pprof server
	pprofSrv := startSecurePprof()
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = pprofSrv.Shutdown(shutdownCtx)
	}()

	// Database Connection Pool Initialization
	db, err := postgres.NewDB(ctx, &cfg.Database)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer db.Close()

	// Initialize repositories & services
	stateRepo := postgres.NewStateRepository(db.Pool)
	lockRepo := postgres.NewLockRepository(db.Pool)

	stateSvc := service.NewStateService(stateRepo)
	lockSvc := service.NewLockService(lockRepo)

	// Register Prometheus metrics
	metrics.Init(db.Pool)

	pingFunc := func(c context.Context) error {
		return db.Pool.Ping(c)
	}

	handler := api.NewHandler(stateSvc, lockSvc, pingFunc)
	router := api.SetupRouter(configMgr, handler)

	// Initialize signals handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	var srv *http.Server
	var redirectSrv *http.Server

	if cfg.HTTPS.Enabled {
		cacheDir := cfg.HTTPS.CacheDir
		if err := os.MkdirAll(cacheDir, 0700); err != nil {
			return fmt.Errorf("failed to create cache dir: %w", err)
		}

		m := &autocert.Manager{
			Prompt:     autocert.AcceptTOS,
			HostPolicy: autocert.HostWhitelist(cfg.HTTPS.Domains...),
			Cache:      autocert.DirCache(cacheDir),
			Email:      cfg.HTTPS.Email,
		}

		// Port 80 to 443 Redirector
		redirectEngine := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			target := "https://" + r.Host + r.URL.Path
			if len(r.URL.RawQuery) > 0 {
				target += "?" + r.URL.RawQuery
			}
			// #nosec G710
			http.Redirect(w, r, target, http.StatusMovedPermanently)
		})

		redirectSrv = &http.Server{
			Addr:              cfg.Server.ListenHTTP,
			Handler:           redirectEngine,
			ReadHeaderTimeout: 5 * time.Second,
		}

		go func() {
			logger.Log.Info("Starting HTTP-to-HTTPS redirector...", zap.String("addr", cfg.Server.ListenHTTP))
			if err := redirectSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				logger.Log.Error("HTTP redirector failed", zap.Error(err))
			}
		}()

		srv = &http.Server{
			Addr:         cfg.Server.Listen,
			Handler:      router,
			TLSConfig:    m.TLSConfig(),
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 30 * time.Second,
			IdleTimeout:  120 * time.Second,
		}

		go func() {
			logger.Log.Info("Starting HTTPS server with Let's Encrypt...", zap.String("addr", cfg.Server.Listen))
			if err := srv.ListenAndServeTLS("", ""); err != nil && !errors.Is(err, http.ErrServerClosed) {
				logger.Log.Error("HTTPS server failed to run", zap.Error(err))
			}
		}()
	} else {
		// Non-HTTPS HTTP fallback
		srv = &http.Server{
			Addr:         cfg.Server.Listen,
			Handler:      router,
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 30 * time.Second,
			IdleTimeout:  120 * time.Second,
		}

		go func() {
			logger.Log.Info("Starting HTTP server (TLS disabled)...", zap.String("addr", cfg.Server.Listen))
			if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				logger.Log.Error("HTTP server failed to run", zap.Error(err))
			}
		}()
	}

	// Loop to catch reloads (SIGHUP) and graceful shutdown (SIGINT/SIGTERM)
	for {
		sig := <-sigChan
		switch sig {
		case syscall.SIGHUP:
			logger.Log.Info("SIGHUP caught, reloading configuration...")
			newCfg, err := config.Load(configPath)
			if err != nil {
				logger.Log.Error("Failed to reload configuration", zap.Error(err))
				continue
			}

			configMgr.Update(newCfg)

			if err := logger.SetLevel(newCfg.Logging.Level); err != nil {
				logger.Log.Error("Failed to reload log level", zap.Error(err))
			} else {
				logger.Log.Info("Log level reloaded successfully", zap.String("level", newCfg.Logging.Level))
			}

		case syscall.SIGINT, syscall.SIGTERM:
			logger.Log.Info("Termination signal received, initiating graceful shutdown...")

			shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			if redirectSrv != nil {
				_ = redirectSrv.Shutdown(shutdownCtx)
			}
			if err := srv.Shutdown(shutdownCtx); err != nil {
				logger.Log.Error("Server shutdown failed", zap.Error(err))
			}

			logger.Log.Info("Server shutdown completed cleanly.")
			return nil
		}
	}
}

func runMigrate(configPath string, direction string) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	dsn := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
		cfg.Database.User,
		cfg.Database.Password,
		cfg.Database.Host,
		cfg.Database.Port,
		cfg.Database.Database,
		cfg.Database.SSLMode,
	)

	d, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return fmt.Errorf("failed to load embedded migrations: %w", err)
	}

	m, err := migrate.NewWithSourceInstance("iofs", d, dsn)
	if err != nil {
		return fmt.Errorf("failed to create migrate instance: %w", err)
	}
	defer func() { _, _ = m.Close() }()

	switch direction {
	case "up":
		if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
			return err
		}
		fmt.Println("Migrated up successfully.")
	case "down":
		if err := m.Down(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
			return err
		}
		fmt.Println("Migrated down successfully.")
	}

	return nil
}
