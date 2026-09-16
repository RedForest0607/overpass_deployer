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
	"studio-backend/internal/localstore"
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
	artifactStore, err := localstore.New(paths.Artifacts, paths.Temporary)
	if err != nil {
		return err
	}
	packageStore, err := localstore.New(paths.Packages, paths.Temporary)
	if err != nil {
		return err
	}
	deliveryRepo := sqlite.NewDeliveryRepository(db)
	delivery, err := service.NewDelivery(projects, deliveryRepo, artifactStore, packageStore, "local-user", cfg.JARBytes, cfg.TextArtifactBytes, cfg.PreviewBytes)
	if err != nil {
		return err
	}
	artifactRefs, packageRefs, err := deliveryRepo.ReferencedBlobKeys(startupContext)
	if err != nil {
		return fmt.Errorf("load referenced blobs: %w", err)
	}
	cleanupBefore := time.Now().UTC().Add(-24 * time.Hour)
	removedArtifacts, err := artifactStore.CleanupUnreferenced(artifactRefs, cleanupBefore)
	if err != nil {
		return fmt.Errorf("cleanup orphan artifacts: %w", err)
	}
	removedPackages, err := packageStore.CleanupUnreferenced(packageRefs, cleanupBefore)
	if err != nil {
		return fmt.Errorf("cleanup orphan packages: %w", err)
	}
	removedTemporary, err := artifactStore.CleanupTemporary(cleanupBefore)
	if err != nil {
		return fmt.Errorf("cleanup temporary blobs: %w", err)
	}
	if removedArtifacts+removedPackages+removedTemporary > 0 {
		logger.Info("orphan blobs cleaned", "artifacts", removedArtifacts, "packages", removedPackages, "temporary", removedTemporary)
	}
	worker, err := service.NewGenerationWorker(delivery, logger, 250*time.Millisecond)
	if err != nil {
		return err
	}
	handler, err := api.NewHandler(api.Dependencies{Config: cfg, Logger: logger, Ready: db, Projects: projects, Delivery: delivery})
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

	runtimeContext, stopRuntime := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopRuntime()
	workerErrors := make(chan error, 1)
	go func() { workerErrors <- worker.Run(runtimeContext) }()

	serverErrors := make(chan error, 1)
	go func() {
		logger.Info("studio api listening", "address", cfg.Address)
		serverErrors <- server.ListenAndServe()
	}()

	select {
	case err := <-serverErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("serve HTTP: %w", err)
		}
		return nil
	case err := <-workerErrors:
		if err != nil {
			return fmt.Errorf("generation worker stopped: %w", err)
		}
		return errors.New("generation worker stopped unexpectedly")
	case <-runtimeContext.Done():
	}

	shutdownContext, cancelShutdown := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelShutdown()
	if err := server.Shutdown(shutdownContext); err != nil {
		return fmt.Errorf("shutdown HTTP server: %w", err)
	}
	if err := <-serverErrors; !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serve HTTP after shutdown: %w", err)
	}
	if err := <-workerErrors; err != nil {
		return fmt.Errorf("stop generation worker: %w", err)
	}
	return nil
}
