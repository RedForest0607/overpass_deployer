package compose

import (
	"errors"

	"go-deployer/internal/config"
	"go-deployer/pkg/logger"
)

type Options struct {
	DryRun bool
}

// Run exposes only a non-mutating plan in Phase 1. A missing --dry-run is
// rejected explicitly so incomplete remote apply behavior cannot be mistaken
// for a deployment.
func Run(cfg *config.Config, options Options) error {
	plan, err := BuildPlan(cfg)
	if err != nil {
		return err
	}
	if !options.DryRun {
		return errors.New("compose apply is not available in Phase 1; use --dry-run")
	}
	for _, action := range plan.Actions {
		scope := action.Server
		if action.Project != "" {
			scope += "/" + action.Project
		}
		logger.Info(scope, "DRY-RUN [%s] %s", action.Kind, action.Summary)
	}
	return nil
}
