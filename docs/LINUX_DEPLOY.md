# Linux Deploy Guide

How to run the bi8s stack on a Linux host (tested on Ubuntu 22.04 / 24.04) using Docker Compose.

---

## Prerequisites

### 1. Install Docker Engine

```bash
curl -fsSL https://get.docker.com | sh
sudo usermod -aG docker $USER          # log out and back in after this
sudo systemctl enable --now docker
```

Verify:

```bash
docker version        # Engine 24+
docker compose version  # Compose v2.x (plugin — note: no hyphen)
```

---

## Clone & Configure

```bash
git clone https://github.com/xanderbilla/bi8s-go.git
cd bi8s-go
```

Copy the production env file and fill in the required values:

```bash
cp .env.example .env.bi8s
$EDITOR .env.bi8s
```

**Required variables** (no defaults):

| Variable                           | Notes                                         |
| ---------------------------------- | --------------------------------------------- |
| `APP_ENV`                          | `production`                                  |
| `AWS_REGION`                       | e.g. `us-east-1`                              |
| `S3_BUCKET`                        | B2 / S3 bucket name                           |
| `DYNAMODB_CONTENT_TABLE`           |                                               |
| `DYNAMODB_PERSON_TABLE`            |                                               |
| `DYNAMODB_ATTRIBUTE_TABLE`         |                                               |
| `DYNAMODB_CONTENT_CAST_TABLE`      |                                               |
| `DYNAMODB_CONTENT_ATTRIBUTE_TABLE` |                                               |
| `CORS_ALLOWED_ORIGINS`             | e.g. `https://bi8s.example.com`               |
| `DOMAIN`                           | root domain for Traefik labels                |
| `GRAFANA_ADMIN_PASSWORD`           | obs profile only                              |
| `TUNNEL_TOKEN`                     | Cloudflare tunnel token (tunnel profile only) |

See [docs/configuration.md](configuration.md) for all variables.

---

## Running the Stack

### Base stack (API + Redis + Traefik)

```bash
docker compose -f infra/docker/docker-compose.yml \
  --env-file .env.bi8s \
  up -d
```

### With Cloudflare Tunnel

```bash
docker compose -f infra/docker/docker-compose.yml \
  --env-file .env.bi8s \
  --profile tunnel \
  up -d
```

### With Observability stack

> **RAM note:** The obs profile adds ~1.2 GB. On a 4 GB host, enable it only when actively debugging.

```bash
docker compose -f infra/docker/docker-compose.yml \
  --env-file .env.bi8s \
  --profile obs \
  up -d
```

### Full stack (tunnel + observability)

```bash
docker compose -f infra/docker/docker-compose.yml \
  --env-file .env.bi8s \
  --profile tunnel --profile obs \
  up -d
```

---

## Updating the Image

```bash
docker compose -f infra/docker/docker-compose.yml \
  --env-file .env.bi8s \
  pull api

docker compose -f infra/docker/docker-compose.yml \
  --env-file .env.bi8s \
  up -d api          # zero-downtime rolling update for the api service
```

---

## Memory Limits

The `api` container has a `mem_limit` to prevent OOM-killing other services. Defaults in the compose file:

| Variable        | Default | Recommended (4 GB host) |
| --------------- | ------- | ----------------------- |
| `API_MEM_LIMIT` | `1.5g`  | `1.5g`                  |

Override in `.env.bi8s` as needed.

---

## Firewall

Only port 80 needs to be open publicly. If you are using Cloudflare Tunnel, no inbound ports are needed at all.

```bash
# UFW example
sudo ufw allow 80/tcp     # Traefik ingress
sudo ufw enable
```

If NOT using Cloudflare Tunnel, also open 443 and configure TLS (see [docs/deployment.md](deployment.md)).

---

## Cloudflare Tunnel Setup

1. Create a tunnel in the [Cloudflare Zero Trust dashboard](https://one.dash.cloudflare.com).
2. Add public hostnames pointing to `http://traefik:80`:
   - `bi8s.example.com` → `http://traefik:80`
   - `api.bi8s.example.com` → `http://traefik:80`
   - `ops.bi8s.example.com` → `http://traefik:80` (Grafana — protect with Access policy)
3. Copy the tunnel token into `.env.bi8s`:
   ```
   TUNNEL_TOKEN=eyJ...
   ```
4. Start with `--profile tunnel`.

---

## Checking Logs

```bash
# All services
docker compose -f infra/docker/docker-compose.yml logs -f

# Just the API
docker compose -f infra/docker/docker-compose.yml logs -f api
```

---

## Health Check

```bash
curl http://localhost/v1/livez      # via Traefik on port 80
curl http://localhost/v1/readyz
```

Expected response: `200 OK` with `{"status":"ok"}`.

---

## Stopping the Stack

```bash
docker compose -f infra/docker/docker-compose.yml \
  --env-file .env.bi8s \
  down
```

Add `--volumes` to also remove named volumes (Prometheus data, Grafana data, etc.):

```bash
docker compose -f infra/docker/docker-compose.yml \
  --env-file .env.bi8s \
  down --volumes
```
