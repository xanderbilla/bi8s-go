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

func TestShutdown_OrderingAndBounds(t *testing.T) {
	t.Setenv("SHUTDOWN_TIMEOUT_SECONDS", "5")
	t.Setenv("ENCODER_DRAIN_TIMEOUT_SECONDS", "5")

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

	start := time.Now()
	if err := shutdown(srv, application); err != nil {
		t.Fatalf("shutdown returned error: %v", err)
	}
	shutdownReturnedAt := time.Now()

	select {
	case <-serverDone:
	case <-time.After(2 * time.Second):
		t.Fatal("http server goroutine did not exit after Shutdown")
	}

	select {
	case <-readyWatcherDone:
	case <-time.After(2 * time.Second):
		t.Fatal("readiness watcher never observed ready=false")
	}

	select {
	case <-respDone:
	case <-time.After(2 * time.Second):
		t.Fatal("in-flight request did not complete")
	}

	readyFalse := readyFalseAt.Load()
	handlerEnded := handlerEndedAt.Load()

	if readyFalse == 0 {
		t.Fatal("readyFalseAt was never recorded")
	}
	if handlerEnded == 0 {
		t.Fatal("handlerEndedAt was never recorded")
	}

	if readyFalse > handlerEnded {
		t.Errorf("readiness flipped to false AFTER handler ended: readyFalse=%d handlerEnded=%d",
			readyFalse, handlerEnded)
	}

	if handlerEnded > shutdownReturnedAt.UnixNano() {
		t.Errorf("handler ended AFTER shutdown returned: handlerEnded=%d shutdownReturnedAt=%d",
			handlerEnded, shutdownReturnedAt.UnixNano())
	}

	if err := rdb.Ping(context.Background()).Err(); err == nil {
		t.Error("redis client expected to be closed after shutdown, ping succeeded")
	}

	elapsed := shutdownReturnedAt.Sub(start)
	if elapsed > 3*time.Second {
		t.Errorf("shutdown exceeded bounded duration: %s", elapsed)
	}
}

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
