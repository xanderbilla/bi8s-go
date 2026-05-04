# Contributing to bi8s-go

Thanks for your interest. This document describes how to propose a change.

## Code of Conduct

By participating you agree to abide by the [Code of Conduct](CODE_OF_CONDUCT.md).

## Getting Started

1. Fork and clone the repository.
2. Read [docs/LOCAL_DEVELOPMENT.md](docs/LOCAL_DEVELOPMENT.md) to bootstrap a local environment.
3. Verify the baseline builds and tests pass before changing anything:

   ```sh
   make test
   make build
   ```

4. Create a feature branch off `dev`:

   ```sh
   git checkout dev
   git pull --ff-only
   git checkout -b feat/<short-description>
   ```

## Branching Model

- `main` - production-ready, protected
- `dev` - integration branch, default target for PRs
- `feat/*`, `fix/*`, `chore/*`, `docs/*` - short-lived topic branches

## Commit Style

Use [Conventional Commits](https://www.conventionalcommits.org):

```
<type>(<scope>): <subject>

<body>
```

Common types: `feat`, `fix`, `chore`, `docs`, `refactor`, `test`, `perf`, `ci`.

## Required Quality Gates

All of these must pass locally and in CI before a PR can merge:

| Check         | Command                 |
| ------------- | ----------------------- |
| Format        | `make format`           |
| Vet           | `make vet`              |
| Lint          | `make lint`             |
| Static check  | `make staticcheck`      |
| Vulnerability | `make govulncheck`      |
| Tests + race  | `make test`             |
| Build         | `make build`            |
| OpenAPI       | `make openapi-validate` |

`make quality` runs the full bundle.

## Adding or Changing an Endpoint

1. Update the contract in [docs/openapi.yaml](docs/openapi.yaml) first.
2. Run `make openapi-sync` to mirror the contract into the embedded asset.
3. Implement the handler under `app/internal/http/`.
4. Add unit tests next to the handler and parser.
5. Add an integration test under `test/integration/` if it touches AWS or Redis.
6. Update [docs/API.md](docs/API.md) and [CHANGELOG.md](CHANGELOG.md).

## Pull Requests

- One logical change per PR. Keep diffs small.
- Link related issues with `Closes #<id>`.
- Fill in the PR template (testing performed, risk, rollback).
- A maintainer will review within a few business days.

## Reporting Bugs

Open an issue with: expected behaviour, actual behaviour, reproduction steps,
environment (OS, Go version, deployment target), and log/trace excerpts.

## Reporting Security Issues

See [SECURITY.md](SECURITY.md). Do not open a public issue.
