package localonly

import (
	"net/http"

	"github.com/xanderbilla/bi8s-go/internal/errs"
)

// Middleware blocks any request that was proxied through Cloudflare.
// Requests arriving via api.emm4bi8s.dev always carry CF-Connecting-IP and
// CF-Ray headers injected by Cloudflare's edge. Direct localhost access
// carries neither, so admin (/a/*) endpoints remain reachable only from
// within the local network.
func Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("CF-Connecting-IP") != "" || r.Header.Get("CF-Ray") != "" {
				errs.Write(w, r, &errs.APIError{
					Status:  http.StatusForbidden,
					Code:    errs.CodeForbidden,
					Message: "Admin endpoints are only accessible from localhost",
				})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
