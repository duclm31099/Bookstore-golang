package bootstrap

import (
	"context"
	"time"

	"go.uber.org/zap"
)

type Runner interface {
	Run(ctx context.Context) error
}

type WorkerApp struct {
	runner          Runner
	logger          *zap.Logger
	shutdownTimeout time.Duration
}

func NewWorkerApp(
	runner Runner,
	logger *zap.Logger,
	shutdownTimeout time.Duration,
) *WorkerApp {
	return &WorkerApp{
		runner:          runner,
		logger:          logger,
		shutdownTimeout: shutdownTimeout,
	}
}

func (a *WorkerApp) Run(ctx context.Context) error {
	a.logger.Info("starting worker app")
	return a.runner.Run(ctx)
}
