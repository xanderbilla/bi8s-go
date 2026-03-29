# Docker and CI Workflow

This document explains how to run, build, and publish the bi8s-go API using the new Docker, Make, and GitHub Actions setup.

## Files Added

- `Dockerfile`: multi-stage production image build.
- `docker-compose.yml`: run the API container with live AWS DynamoDB.
- `Makefile`: common developer and CI image commands.
- `postman/bi8s-go.collection.json`: Postman collection for all current API routes.
- `.github/workflows/docker-publish.yml`: CI workflow to build and push image.

## Local Development

Use Air (default):

```sh
make
```

Run with Docker Compose:

```sh
make docker
```

Stop Compose services:

```sh
make stop
```

## Build Images

Build a local single-platform image and load it into Docker:

```sh
make build IMAGE_TAG=dev
```

Build and push multi-platform image:

```sh
make publish IMAGE_TAG=v1.0.0
```

Default image name is `docker.io/xanderbilla/go/bi8s`, and default platforms are `linux/amd64,linux/arm64`.

## GitHub Actions Workflow

Workflow file: `.github/workflows/docker-publish.yml`

Trigger conditions:

- Push to `master` branch
- Only when files under `cmd/**`, `internal/**`, or `.air.toml` change
- Manual trigger via `workflow_dispatch`

The workflow builds and pushes:

- `docker.io/xanderbilla/go/bi8s:latest`
- `docker.io/xanderbilla/go/bi8s:<commit-sha>`

## Required Repository Secrets

Set the following repository secrets:

- `DOCKER_REGISTRY` = `docker.io`
- `DOCKER_USERNAME` = your Docker Hub username
- `DOCKER_PASSWORD` = Docker Hub access token

## Postman Collection

Import:

- `postman/bi8s-go.collection.json`

It includes all current routes:

- `GET /v1/health`
- `GET /v1/movies`
- `GET /v1/movies/{movieId}`
- `POST /v1/movies`
- `DELETE /v1/movies/{movieId}`
