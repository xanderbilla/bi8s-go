package http

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"github.com/xanderbilla/bi8s-go/internal/ctxutil"
	"github.com/xanderbilla/bi8s-go/internal/http/middleware/ratelimit"
)

var permissionsPolicy = strings.Join([]string{
	"accelerometer=()",
	"autoplay=()",
	"camera=()",
	"display-capture=()",
	"encrypted-media=()",
	"fullscreen=(self)",
	"geolocation=()",
	"gyroscope=()",
	"magnetometer=()",
	"microphone=()",
	"midi=()",
	"payment=()",
	"picture-in-picture=()",
	"publickey-credentials-get=()",
	"screen-wake-lock=()",
	"sync-xhr=()",
	"usb=()",
	"xr-spatial-tracking=()",
}, ", ")

func RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = middleware.GetReqID(r.Context())
		}
		if requestID == "" {
			requestID = uuid.NewString()
		}
		ctx := ctxutil.WithRequestID(r.Context(), requestID)
		ctx = context.WithValue(ctx, middleware.RequestIDKey, requestID)
		w.Header().Set("X-Request-ID", requestID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func SecureHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Cross-Origin-Opener-Policy", "same-origin")
		h.Set("Cross-Origin-Resource-Policy", "same-origin")
		h.Set("Permissions-Policy", permissionsPolicy)

		h["Server"] = nil
		if r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
			h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		next.ServeHTTP(w, r)
	})
}

func MaxBytesJSON(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Body != nil &&
				r.ContentLength != 0 &&
				!strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), "multipart/") {
				r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			}
			next.ServeHTTP(w, r)
		})
	}
}

type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusRecorder) Write(b []byte) (int, error) {
	if s.status == 0 {
		s.status = http.StatusOK
	}
	n, err := s.ResponseWriter.Write(b)
	s.bytes += n
	return n, err
}

func RequestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if (path == "/v1/livez" || path == "/v1/readyz") &&
			!slog.Default().Enabled(r.Context(), slog.LevelDebug) {
			next.ServeHTTP(w, r)
			return
		}
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w}
		next.ServeHTTP(rec, r)

		if rec.status == 0 {
			rec.status = http.StatusOK
		}

		attrs := []any{
			"request_id", middleware.GetReqID(r.Context()),
			"method", r.Method,
			"path", path,
			"status", rec.status,
			"bytes", rec.bytes,
			"duration_ms", time.Since(start).Milliseconds(),
			"remote_addr", ratelimit.GetClientIP(r),
		}
		if ua := r.UserAgent(); ua != "" {
			attrs = append(attrs, "user_agent", ua)
		}
		if q := r.URL.RawQuery; q != "" {
			attrs = append(attrs, "query", q)
		}
		switch {
		case rec.status >= 500:
			slog.Error("http_request", attrs...)
		case rec.status >= 400:
			slog.Warn("http_request", attrs...)
		default:
			slog.Info("http_request", attrs...)
		}
	})
}
