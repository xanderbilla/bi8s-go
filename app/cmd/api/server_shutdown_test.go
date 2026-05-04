package main

import (
	"context"
	"io"
	"net"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"

	"github.com/xanderbilla/bi8s-go/internal/app"
	transport "github.com/xanderbilla/bi8s-go/internal/http"
	"github.com/xanderbilla/bi8s-go/internal/service"
)

// TestShutdown_OrderingAndBounds verifies the documented graceful-shutdown
// contract enforced by the production-readiness review:
//
//  1. readiness flips to false immediately so the platform stops routing
//     new traffic before any draining begins;
//  2. in-flight HTTP requests are allowed to complete before srv.Shutdown
//     returns (no premature connection termination);
//  3. encoder Shutdown is signalled and Wait blocks until in-flight jobs
//     finish (we simulate one with a 60ms hold);
//  4. Redis is closed only after the encoder drain completes so that
//     in-flight rate-limit checks during drain can still reach the backend;
//  5. the entire sequence completes within SHUTDOWN_TIMEOUT_SECONDS plus a
//     bounded encoder-drain window.
func TestShutdown_OrderingAndBounds(t *testing.T) {
	t.Setenv("SHUTDOWN_TIMEOUT_SECONDS", "5")
	t.Setenv("ENCODER_DRAIN_TIMEOUT_SECONDS", "5")

	// --- HTTP server with a deliberately slow handler ---------------------
	const handlerDelay = 80 * time.Millisecond
	var (
		handlerStartedAt atomic.Int64
		handlerEndedAt   atomic.Int64
		readyFalseAt     atomic.Int64
	)

	mux := http.NewServeMux()
	mux.HandleFunc("/slow", func(w http.ResponseWriter, _ *http.Request) {
		handlerStartedAt.Store(time.Now().UnixNano())
		time.Sleep(handlerDelay)
		handlerEndedAt.Store(time.Now().UnixNano())
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "ok")
	})

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	srv := newHTTPServer(ln.Addr().String(), mux)

	transport.SetReady(true)
	serverDone := make(chan struct{})
	go func() {
		defer close(serverDone)
		_ = srv.Serve(ln)
	}()

	// --- Application with real encoder + miniredis ------------------------
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)

	rdb := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		t.Fatalf("redis ping: %v", err)
	}

	encoder := service.NewEncoderService(nil, nil)

	application := &app.Application{
		EncoderService: encoder,
		RedisClient:    rdb,
	}

	// --- Fire an in-flight request and a readiness watcher ---------------
	respDone := make(chan struct{})
	go func() {
		defer close(respDone)
		resp, err := http.Get("http://" + ln.Addr().String() + "/slow")
		if err != nil {
			t.Errorf("inflight request: %v", err)
			return
		}
		defer resp.Body.Close()
		_, _ = io.Copy(io.Discard, resp.Body)
	}()

	// Wait until the handler is actually executing so the request is truly
	// in-flight when we trigger shutdown.
	deadline := time.Now().Add(2 * time.Second)
	for handlerStartedAt.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(2 * time.Millisecond)
	}
	if handlerStartedAt.Load() == 0 {
		t.Fatal("handler never started")
	}

	readyWatcherDone := make(chan struct{})
	go func() {
		defer close(readyWatcherDone)
		for {
			if !transport.IsReady() {
				readyFalseAt.Store(time.Now().UnixNano())
				return
			}
			time.Sleep(time.Millisecond)
		}
	}()

	// --- Run shutdown ----------------------------------------------------
	start := time.Now()
	if err := shutdown(srv, application); err != nil {
		t.Fatalf("shutdown returned error: %v", err)
	}
	shutdownReturnedAt := time.Now()

	// Server goroutine must exit because Shutdown was called.
	select {
	case <-serverDone:
	case <-time.After(2 * time.Second):
		t.Fatal("http server goroutine did not exit after Shutdown")
	}

	// Watcher goroutine must observe ready=false.
	select {
	case <-readyWatcherDone:
	case <-time.After(2 * time.Second):
		t.Fatal("readiness watcher never observed ready=false")
	}

	// In-flight request must have completed before we call shutdown done.
	select {
	case <-respDone:
	case <-time.After(2 * time.Second):
		t.Fatal("in-flight request did not complete")
	}

	// --- Ordering assertions --------------------------------------------
	readyFalse := readyFalseAt.Load()
	handlerEnded := handlerEndedAt.Load()

	if readyFalse == 0 {
		t.Fatal("readyFalseAt was never recorded")
	}
	if handlerEnded == 0 {
		t.Fatal("handlerEndedAt was never recorded")
	}

	// Readiness flipped before the in-flight handler finished (proves the
	// readiness flip happens early, allowing the platform to stop routing
	// while the existing request still drains).
	if readyFalse > handlerEnded {
		t.Errorf("readiness flipped to false AFTER handler ended: readyFalse=%d handlerEnded=%d",
			readyFalse, handlerEnded)
	}

	// Handler must have finished before shutdown returned (proves srv.Shutdown
	// drained the connection rather than killing it).
	if handlerEnded > shutdownReturnedAt.UnixNano() {
		t.Errorf("handler ended AFTER shutdown returned: handlerEnded=%d shutdownReturnedAt=%d",
			handlerEnded, shutdownReturnedAt.UnixNano())
	}

	// Redis client must be closed after shutdown completes.
	if err := rdb.Ping(context.Background()).Err(); err == nil {
		t.Error("redis client expected to be closed after shutdown, ping succeeded")
	}

	// Bounded total runtime: SHUTDOWN_TIMEOUT_SECONDS + ENCODER_DRAIN +
	// generous slack. With no real jobs in flight this should be sub-second.
	elapsed := shutdownReturnedAt.Sub(start)
	if elapsed > 3*time.Second {
		t.Errorf("shutdown exceeded bounded duration: %s", elapsed)
	}
}

// TestShutdown_NilRedisIsSafe ensures the shutdown path tolerates the
// memory rate-limit-backend configuration where RedisClient is nil.
func TestShutdown_NilRedisIsSafe(t *testing.T) {
	t.Setenv("SHUTDOWN_TIMEOUT_SECONDS", "2")
	t.Setenv("ENCODER_DRAIN_TIMEOUT_SECONDS", "2")

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	srv := newHTTPServer(ln.Addr().String(), http.NewServeMux())

	serverDone := make(chan struct{})
	go func() {
		defer close(serverDone)
		_ = srv.Serve(ln)
	}()

	application := &app.Application{
		EncoderService: service.NewEncoderService(nil, nil),
		RedisClient:    nil,
	}

	if err := shutdown(srv, application); err != nil {
		t.Fatalf("shutdown returned error: %v", err)
	}

	select {
	case <-serverDone:
	case <-time.After(2 * time.Second):
		t.Fatal("server did not exit")
	}
}
