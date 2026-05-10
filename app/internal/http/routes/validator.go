package routes

import (
	"errors"
	"fmt"
	"net/netip"
	"strings"
)

func Validate(cfg Config, handlers HandlerRegistry, mws MiddlewareRegistry) error {
	var errs []error

	if cfg.Version != 1 {
		errs = append(errs, fmt.Errorf("routes: unsupported config version %d (want 1)", cfg.Version))
	}

	for _, name := range cfg.Defaults.Middlewares {
		token := strings.TrimPrefix(name, "+")
		if _, ok := mws[token]; !ok {
			errs = append(errs, fmt.Errorf("routes: defaults middleware %q is not registered", token))
		}
	}
	for _, cidr := range cfg.Defaults.AllowedIPs {
		if perr := validateCIDR(cidr); perr != nil {
			errs = append(errs, fmt.Errorf("routes: defaults allowedIPs: %w", perr))
		}
	}

	seen := make(map[string]struct{}, len(cfg.Routes))
	pathMethod := make(map[string]struct{}, len(cfg.Routes))

	for i, r := range cfg.Routes {
		ctx := fmt.Sprintf("routes[%d] (%s)", i, r.ID)

		if strings.TrimSpace(r.ID) == "" {
			errs = append(errs, fmt.Errorf("%s: id is required", ctx))
		} else if _, dup := seen[r.ID]; dup {
			errs = append(errs, fmt.Errorf("%s: duplicate id", ctx))
		} else {
			seen[r.ID] = struct{}{}
		}

		if strings.TrimSpace(r.Route) == "" || !strings.HasPrefix(r.Route, "/") {
			errs = append(errs, fmt.Errorf("%s: route must be a non-empty absolute path", ctx))
		}

		method := strings.ToUpper(strings.TrimSpace(r.Method))
		if _, ok := supportedMethods[method]; !ok {
			errs = append(errs, fmt.Errorf("%s: method %q is not supported", ctx, r.Method))
		} else {
			key := method + " " + r.Route
			if _, dup := pathMethod[key]; dup {
				errs = append(errs, fmt.Errorf("%s: duplicate (method, route): %s", ctx, key))
			} else {
				pathMethod[key] = struct{}{}
			}
		}

		if strings.TrimSpace(r.Handler) == "" {
			errs = append(errs, fmt.Errorf("%s: handler is required", ctx))
		} else if _, ok := handlers[r.Handler]; !ok {
			errs = append(errs, fmt.Errorf("%s: handler %q is not registered", ctx, r.Handler))
		}

		for _, name := range r.Middlewares {
			token := strings.TrimPrefix(name, "+")
			if _, ok := mws[token]; !ok {
				errs = append(errs, fmt.Errorf("%s: middleware %q is not registered", ctx, token))
			}
		}

		for _, cidr := range r.AllowedIPs {
			if perr := validateCIDR(cidr); perr != nil {
				errs = append(errs, fmt.Errorf("%s: allowedIPs: %w", ctx, perr))
			}
		}

		if r.RateLimit != nil {
			if r.RateLimit.Requests < 0 || r.RateLimit.WindowSeconds < 0 {
				errs = append(errs, fmt.Errorf("%s: rateLimit requests/windowSeconds must be >= 0", ctx))
			}
		}
		if r.Cache != nil && r.Cache.TTLSeconds < 0 {
			errs = append(errs, fmt.Errorf("%s: cache.ttlSeconds must be >= 0", ctx))
		}
		if r.CacheTTLSec < 0 {
			errs = append(errs, fmt.Errorf("%s: cacheTTLSeconds must be >= 0", ctx))
		}
		if r.DisabledResponse != "" &&
			r.DisabledResponse != DisabledNotFound &&
			r.DisabledResponse != DisabledForbidden {
			errs = append(errs, fmt.Errorf("%s: disabledResponse must be one of: notfound, forbidden", ctx))
		}
	}

	return errors.Join(errs...)
}

func validateCIDR(s string) error {
	s = strings.TrimSpace(s)
	if s == "" {
		return errors.New("empty CIDR")
	}
	if strings.Contains(s, "/") {
		if _, err := netip.ParsePrefix(s); err != nil {
			return fmt.Errorf("invalid CIDR %q: %w", s, err)
		}
		return nil
	}
	if _, err := netip.ParseAddr(s); err != nil {
		return fmt.Errorf("invalid IP %q: %w", s, err)
	}
	return nil
}
