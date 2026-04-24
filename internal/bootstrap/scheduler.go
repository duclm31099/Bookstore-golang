package bootstrap

import (
	"context"
	"time"

	"go.uber.org/zap"
)

type SchedulerApp struct {
	runner          Runner
	logger          *zap.Logger
	shutdownTimeout time.Duration
}

func NewSchedulerApp(
	runner Runner,
	logger *zap.Logger,
	shutdownTimeout time.Duration,
) *SchedulerApp {
	return &SchedulerApp{
		runner:          runner,
		logger:          logger,
		shutdownTimeout: shutdownTimeout,
	}
}

func (a *SchedulerApp) Run(ctx context.Context) error {
	a.logger.Info("starting scheduler app")
	return a.runner.Run(ctx)
}
