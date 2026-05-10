package http

import (
	"context"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"github.com/xanderbilla/bi8s-go/internal/ctxutil"
	"github.com/xanderbilla/bi8s-go/internal/http/middleware/ratelimit"
	"github.com/xanderbilla/bi8s-go/internal/http/respwriter"
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
		requestID := sanitizeRequestID(r.Header.Get("X-Request-ID"))
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

func MaxBytesMultipart(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Body != nil &&
				r.ContentLength != 0 &&
				strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), "multipart/") {
				r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			}
			next.ServeHTTP(w, r)
		})
	}
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
		rec := respwriter.New(w)
		next.ServeHTTP(rec, r)

		if rec.Status == 0 {
			rec.Status = http.StatusOK
		}

		attrs := []any{
			"request_id", middleware.GetReqID(r.Context()),
			"method", r.Method,
			"path", path,
			"status", rec.Status,
			"bytes", rec.Bytes,
			"duration_ms", time.Since(start).Milliseconds(),
			"remote_addr", ratelimit.GetClientIP(r),
		}
		if ua := r.UserAgent(); ua != "" {
			attrs = append(attrs, "user_agent", ua)
		}
		if q := r.URL.RawQuery; q != "" {
			attrs = append(attrs, "query", redactQuery(r.URL.Query()).Encode())
		}
		switch {
		case rec.Status >= 500:
			slog.Error("http_request", attrs...)
		case rec.Status >= 400:
			slog.Warn("http_request", attrs...)
		default:
			slog.Info("http_request", attrs...)
		}
	})
}

func sanitizeRequestID(v string) string {
	v = strings.TrimSpace(v)
	if v == "" || len(v) > 128 {
		return ""
	}
	for _, r := range v {
		if r > unicode.MaxASCII {
			return ""
		}
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '-' && r != '_' && r != '.' && r != ':' {
			return ""
		}
	}
	return v
}

func redactQuery(values url.Values) url.Values {
	if len(values) == 0 {
		return values
	}
	out := make(url.Values, len(values))
	for key, vals := range values {
		if isSensitiveQueryKey(key) {
			out[key] = []string{"[REDACTED]"}
			continue
		}
		cloned := make([]string, len(vals))
		copy(cloned, vals)
		out[key] = cloned
	}
	return out
}

func isSensitiveQueryKey(key string) bool {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "token", "access_token", "id_token", "refresh_token", "api_key", "apikey", "key", "signature", "sig", "password", "secret", "authorization":
		return true
	default:
		return false
	}
}
