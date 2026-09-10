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

	"studio-backend/internal/api"
	"studio-backend/internal/config"
	"studio-backend/internal/service"
	"studio-backend/internal/sqlite"
	"studio-backend/internal/storage"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(os.Args[1:], os.LookupEnv, logger); err != nil {
		logger.Error("studio api stopped", "error", err)
		os.Exit(1)
	}
}

func run(args []string, lookupEnv config.LookupEnv, logger *slog.Logger) error {
	cfg, err := config.Parse(args, lookupEnv)
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}
	paths, err := storage.Prepare(cfg.DataDir)
	if err != nil {
		return err
	}

	startupContext, cancelStartup := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelStartup()
	db, err := sqlite.Open(startupContext, paths.Database)
	if err != nil {
		return err
	}
	defer db.Close()

	projects, err := service.NewProjects(sqlite.NewProjectRepository(db), "local-user", cfg.TopologyNodes, cfg.TopologyEdges)
	if err != nil {
		return err
	}
	handler, err := api.NewHandler(api.Dependencies{Config: cfg, Logger: logger, Ready: db, Projects: projects})
	if err != nil {
		return err
	}
	server := &http.Server{
		Addr:              cfg.Address,
		Handler:           handler,
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
	}

	serverErrors := make(chan error, 1)
	go func() {
		logger.Info("studio api listening", "address", cfg.Address)
		serverErrors <- server.ListenAndServe()
	}()

	shutdownSignal, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	select {
	case err := <-serverErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("serve HTTP: %w", err)
		}
		return nil
	case <-shutdownSignal.Done():
	}

	shutdownContext, cancelShutdown := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelShutdown()
	if err := server.Shutdown(shutdownContext); err != nil {
		return fmt.Errorf("shutdown HTTP server: %w", err)
	}
	if err := <-serverErrors; !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serve HTTP after shutdown: %w", err)
	}
	return nil
}
