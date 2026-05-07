package ipallow

import (
	"net/http"
	"net/netip"
	"strings"

	"github.com/xanderbilla/bi8s-go/internal/errs"
	"github.com/xanderbilla/bi8s-go/internal/http/middleware/ratelimit"
)

func Allow(cidrs []string) func(http.Handler) http.Handler {
	prefixes := make([]netip.Prefix, 0, len(cidrs))
	for _, raw := range cidrs {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		if !strings.Contains(raw, "/") {
			addr, err := netip.ParseAddr(raw)
			if err != nil {
				continue
			}
			bits := 32
			if addr.Is6() {
				bits = 128
			}
			prefixes = append(prefixes, netip.PrefixFrom(addr, bits))
			continue
		}
		p, err := netip.ParsePrefix(raw)
		if err != nil {
			continue
		}
		prefixes = append(prefixes, p)
	}

	if len(prefixes) == 0 {
		return func(next http.Handler) http.Handler { return next }
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := ratelimit.GetClientIP(r)
			addr, err := netip.ParseAddr(ip)
			if err != nil {
				errs.Write(w, r, &errs.APIError{
					Status:  http.StatusForbidden,
					Code:    errs.CodeForbidden,
					Message: "Access denied",
				})
				return
			}
			for _, p := range prefixes {
				if p.Contains(addr) {
					next.ServeHTTP(w, r)
					return
				}
			}
			errs.Write(w, r, &errs.APIError{
				Status:  http.StatusForbidden,
				Code:    errs.CodeForbidden,
				Message: "Access denied",
			})
		})
	}
}
