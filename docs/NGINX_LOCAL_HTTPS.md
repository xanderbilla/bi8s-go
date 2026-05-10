# Local HTTPS via nginx

This document explains how to run the local stack with TLS terminated by the
nginx container in [`infra/docker`](../infra/docker). Production nginx config
lives in the EC2 user-data template ([`infra/tofu/envs/_shared/user-data.sh.tpl`](../infra/tofu/envs/_shared/user-data.sh.tpl));
the local config in [`infra/docker/nginx/`](../infra/docker/nginx/) (split into
`nginx.conf` + `conf.d/api.conf` + reusable `snippets/*.conf`)
mirrors the security posture (Mozilla intermediate TLS profile, HSTS in prod
only, HTTP→HTTPS redirect).

Generate a local cert with [`scripts/dev/gen-local-cert.sh`](../scripts/dev/gen-local-cert.sh)
(thin `mkcert` wrapper that writes to `infra/docker/nginx/ssl/live/`).

## Sections

- Generating a local cert with `mkcert` (or `openssl` fallback)
- Mounting the cert into the nginx container
- Trusting the local CA on macOS / Linux / WSL
- Verifying the chain (`curl --resolve`, `openssl s_client`)
- Tearing it down cleanly

## Caveats

- Browsers require a CA trust import; `curl -k` is fine for smoke tests.
- The local cert is **not** OCSP-stapled; production is.
- Do not commit cert/key material — `.gitignore` already excludes the dev
  cert directory.

## See also

- [`LOCAL_DEVELOPMENT.md`](LOCAL_DEVELOPMENT.md) — full local stack
- [`SECURITY.md`](SECURITY.md) — TLS profile and HSTS policy
