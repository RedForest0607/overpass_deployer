package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"studio-backend/internal/domain"
	"studio-backend/internal/repository"
)

const defaultGenerationAttempts = 3

type GenerationWorker struct {
	delivery     *Delivery
	logger       *slog.Logger
	pollInterval time.Duration
	jobTimeout   time.Duration
	maxAttempts  int
}

func NewGenerationWorker(delivery *Delivery, logger *slog.Logger, pollInterval time.Duration) (*GenerationWorker, error) {
	if delivery == nil || logger == nil || pollInterval <= 0 {
		return nil, errors.New("generation worker dependencies are required")
	}
	return &GenerationWorker{delivery: delivery, logger: logger, pollInterval: pollInterval, jobTimeout: 5 * time.Minute, maxAttempts: defaultGenerationAttempts}, nil
}

func (w *GenerationWorker) Run(ctx context.Context) error {
	recovered, err := w.delivery.repo.RecoverGenerations(ctx, w.maxAttempts, w.delivery.now().UTC())
	if err != nil {
		if ctx.Err() != nil {
			return nil
		}
		return fmt.Errorf("recover generation jobs: %w", err)
	}
	if recovered > 0 {
		w.logger.Info("generation jobs recovered", "count", recovered)
	}

	timer := time.NewTimer(0)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-timer.C:
			if err := w.processAvailable(ctx); err != nil && !errors.Is(err, context.Canceled) {
				w.logger.Error("generation worker cycle failed", "error_type", fmt.Sprintf("%T", err))
			}
			timer.Reset(w.pollInterval)
		}
	}
}

func (w *GenerationWorker) processAvailable(ctx context.Context) error {
	for {
		generation, err := w.delivery.repo.ClaimNextGeneration(ctx, w.maxAttempts, w.delivery.now().UTC())
		if errors.Is(err, repository.ErrNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		w.processOne(ctx, generation)
	}
}

func (w *GenerationWorker) processOne(parent context.Context, generation domain.Generation) {
	ctx, cancel := context.WithTimeout(parent, w.jobTimeout)
	defer cancel()

	var pkg domain.Package
	var jobErr error
	func() {
		defer func() {
			if recover() != nil {
				jobErr = errors.New("generation worker panic")
				w.logger.Error("generation worker panic", "generationId", generation.ID)
			}
		}()
		pkg, jobErr = w.delivery.BuildGeneration(ctx, generation)
	}()

	now := w.delivery.now().UTC()
	if jobErr == nil {
		_, jobErr = w.delivery.repo.CompleteGeneration(ctx, generation.ID, pkg, now)
	}
	if jobErr == nil {
		w.logger.Info("generation completed", "generationId", generation.ID, "packageId", pkg.ID, "attempt", generation.Attempts)
		return
	}
	if parent.Err() != nil {
		w.logger.Info("generation interrupted for restart recovery", "generationId", generation.ID, "attempt", generation.Attempts)
		return
	}

	failure := safeGenerationFailure(jobErr)
	var invalidArtifact *InvalidArtifact
	if generation.Attempts < w.maxAttempts && !errors.As(jobErr, &invalidArtifact) {
		if retryErr := w.delivery.repo.RequeueGeneration(context.WithoutCancel(parent), generation.ID, failure, now); retryErr != nil {
			w.logger.Error("requeue generation", "generationId", generation.ID, "error_type", fmt.Sprintf("%T", retryErr), "failure_code", failure.Code)
			return
		}
		w.logger.Warn("generation scheduled for retry", "generationId", generation.ID, "attempt", generation.Attempts, "failure_code", failure.Code)
		return
	}
	if failErr := w.delivery.repo.FailGeneration(context.WithoutCancel(parent), generation.ID, failure, now); failErr != nil {
		w.logger.Error("record generation failure", "generationId", generation.ID, "error_type", fmt.Sprintf("%T", failErr), "failure_code", failure.Code)
		return
	}
	w.logger.Warn("generation failed", "generationId", generation.ID, "attempt", generation.Attempts, "failure_code", failure.Code)
}

func safeGenerationFailure(err error) domain.Failure {
	var invalidArtifact *InvalidArtifact
	if errors.As(err, &invalidArtifact) {
		return domain.Failure{Code: "INVALID_ARTIFACT", Message: "artifact 참조 또는 형식이 올바르지 않습니다."}
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return domain.Failure{Code: "GENERATION_TIMEOUT", Message: "패키지 생성 제한 시간을 초과했습니다."}
	}
	return domain.Failure{Code: "GENERATION_FAILED", Message: "패키지 생성 중 내부 오류가 발생했습니다."}
}
