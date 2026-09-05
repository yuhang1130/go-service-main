package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/yuhang1130/go-service-main/internal/foundation/config"
)

type HTTP struct {
	name            string
	server          *http.Server
	shutdownTimeout time.Duration
	logger          *slog.Logger
}

type Runner interface {
	Run(context.Context) error
}

type routeLister interface {
	Routes() []string
}

func New(name string, port int, handler http.Handler, cfg config.Server, logger *slog.Logger) *HTTP {
	return &HTTP{
		name: name,
		server: &http.Server{
			Addr:              ":" + strconv.Itoa(port),
			Handler:           handler,
			ReadHeaderTimeout: cfg.ReadHeaderTimeout,
			ReadTimeout:       cfg.ReadTimeout,
			WriteTimeout:      cfg.WriteTimeout,
			IdleTimeout:       cfg.IdleTimeout,
			MaxHeaderBytes:    cfg.MaxHeaderBytes,
		},
		shutdownTimeout: cfg.ShutdownTimeout,
		logger:          logger,
	}
}

func (s *HTTP) OnShutdown(callback func()) {
	if callback != nil {
		s.server.RegisterOnShutdown(callback)
	}
}

func (s *HTTP) Run(ctx context.Context) error {
	s.logRoutes()
	errorsChannel := make(chan error, 1)
	go func() {
		s.logger.Info("http server started", "server", s.name, "address", s.server.Addr)
		errorsChannel <- s.server.ListenAndServe()
	}()
	select {
	case err := <-errorsChannel:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("%s server: %w", s.name, err)
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), s.shutdownTimeout)
		defer cancel()
		return s.server.Shutdown(shutdownCtx)
	}
}

func (s *HTTP) logRoutes() {
	routes, ok := s.server.Handler.(routeLister)
	if !ok {
		return
	}
	for _, route := range routes.Routes() {
		s.logger.Info("http route registered", "server", s.name, "route", route)
	}
}

func RunAll(ctx context.Context, servers ...Runner) error {
	if len(servers) == 0 {
		return nil
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	errorsChannel := make(chan error, len(servers))
	for _, item := range servers {
		item := item
		go func() { errorsChannel <- item.Run(ctx) }()
	}
	firstError := <-errorsChannel
	cancel()
	errorsSeen := make([]error, 0, len(servers))
	if firstError != nil {
		errorsSeen = append(errorsSeen, firstError)
	}
	for remaining := 1; remaining < len(servers); remaining++ {
		if err := <-errorsChannel; err != nil {
			errorsSeen = append(errorsSeen, err)
		}
	}
	return errors.Join(errorsSeen...)
}
