package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/xanderbilla/bi8s-go/internal/app"
	"github.com/xanderbilla/bi8s-go/internal/env"
	transport "github.com/xanderbilla/bi8s-go/internal/http"
)

// newHTTPServer constructs a *http.Server with timeouts sourced from env.
func newHTTPServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: time.Duration(env.GetInt("HTTP_READ_HEADER_TIMEOUT_SECONDS", 5)) * time.Second,
		ReadTimeout:       time.Duration(env.GetInt("HTTP_READ_TIMEOUT_SECONDS", 30)) * time.Second,
		WriteTimeout:      time.Duration(env.GetInt("HTTP_WRITE_TIMEOUT_SECONDS", 65)) * time.Second,
		IdleTimeout:       time.Duration(env.GetInt("HTTP_IDLE_TIMEOUT_SECONDS", 120)) * time.Second,
		MaxHeaderBytes:    env.GetInt("HTTP_MAX_HEADER_BYTES", 1<<20),
		ErrorLog:          slog.NewLogLogger(slog.Default().Handler(), slog.LevelError),
	}
}

// serve mounts the router, starts the HTTP server, blocks until a shutdown
// signal arrives or the server fails, then performs graceful shutdown and
// drains any in-flight encoding jobs.
func serve(application *app.Application) error {
	mux, closeRouter := transport.Mount(application)
	defer closeRouter()

	srv := newHTTPServer(application.Config.Addr, mux)

	serverErr := make(chan error, 1)
	go func() {
		slog.Info("http server listening", "addr", application.Config.Addr)
		transport.SetReady(true)
		if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-serverErr:
		return err
	case sig := <-quit:
		slog.Info("shutdown signal received", "signal", sig)
	}

	return shutdown(srv, application)
}

// shutdown stops accepting new connections, waits for in-flight requests, and
// then drains the encoder service so background jobs can finish.
func shutdown(srv *http.Server, application *app.Application) error {
	transport.SetReady(false)

	shutdownCtx, cancel := context.WithTimeout(
		context.Background(),
		time.Duration(env.GetInt("SHUTDOWN_TIMEOUT_SECONDS", 30))*time.Second,
	)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("http server shutdown error", "error", err)
	}

	encoderCtx, encoderCancel := context.WithTimeout(
		context.Background(),
		time.Duration(env.GetInt("ENCODER_DRAIN_TIMEOUT_SECONDS", 120))*time.Second,
	)
	defer encoderCancel()

	slog.Info("draining encoding jobs...")
	if err := application.EncoderService.Wait(encoderCtx); err != nil {
		slog.Warn("encoding jobs did not complete within timeout, forcing shutdown", "error", err)
	} else {
		slog.Info("all encoding jobs completed")
	}

	slog.Info("server stopped cleanly")
	return nil
}
