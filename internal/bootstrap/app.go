// internal/bootstrap/app.go
package bootstrap

import (
	"context"
	"errors"
	"net/http"
	"time"

	"go.uber.org/zap"
)

type APIApp struct {
	server          *http.Server
	logger          *zap.Logger
	shutdownTimeout time.Duration
}

func NewAPIApp(
	server *http.Server,
	logger *zap.Logger,
	shutdownTimeout time.Duration,
) *APIApp {
	return &APIApp{
		server:          server,
		logger:          logger,
		shutdownTimeout: shutdownTimeout,
	}
}

func (a *APIApp) Run() error {
	a.logger.Info("starting api server", zap.String("addr", a.server.Addr))

	err := a.server.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	return nil
}

func (a *APIApp) Shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), a.shutdownTimeout)
	defer cancel()

	a.logger.Info("shutting down api server")
	return a.server.Shutdown(ctx)
}
