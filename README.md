# Bi8s (Go)

A REST API built with Go, [chi](https://github.com/go-chi/chi), and DynamoDB on AWS for managing movies and persons (performers/content creators).

## Requirements

- Go 1.25+
- Docker & Docker Compose (for deployment)
- AWS account
- [air](https://github.com/air-verse/air) _(optional, for live reload during development)_

## Environment Variables

| Variable                                      | Description                                                          | Default              |
| --------------------------------------------- | -------------------------------------------------------------------- | -------------------- |
| `APP_ENV`                                     | Runtime environment (`dev`, `prod`)                                  | `prod`               |
| `PORT`                                        | Listen address                                                       | `:8080`              |
| `LOG_LEVEL`                                   | `debug` \| `info` \| `warn` \| `error`                               | `info`               |
| `LOG_ADD_SOURCE`                              | Include source file:line in logs                                     | `false`              |
| `AWS_REGION`                                  | AWS region                                                           | `us-east-1`          |
| `AWS_ACCESS_KEY_ID` / `AWS_SECRET_ACCESS_KEY` | Local-dev only — prefer IAM roles in prod                            | _from IAM_           |
| `DYNAMODB_MOVIE_TABLE`                        | DynamoDB table name for movies                                       | `bi8s-dev`           |
| `DYNAMODB_PERSON_TABLE`                       | DynamoDB table name for persons                                      | `bi8s-person-dev`    |
| `DYNAMODB_ATTRIBUTE_TABLE`                    | DynamoDB table name for attributes                                   | `bi8s-attribute-dev` |
| `DYNAMODB_ENCODER_TABLE`                      | DynamoDB table name for encoder jobs                                 | `bi8s-video-dev`     |
| `DYNAMODB_ENCODER_CONTENT_ID_INDEX`           | GSI used to look up encoder jobs by `contentId`                      | `contentId-index`    |
| `S3_BUCKET`                                   | S3 bucket for source uploads and HLS output                          | _required_           |
| `CORS_ALLOWED_ORIGINS`                        | Comma-separated list of allowed CORS origins                         | `*`                  |
| `CORS_ALLOW_PRIVATE_NETWORK`                  | Allow private-network preflight requests                             | `true`               |
| `TRUSTED_PROXIES`                             | Comma-separated CIDRs of trusted proxies (enables `X-Forwarded-For`) | _empty_              |
| `ENCODER_MAX_CONCURRENT`                      | Max concurrent ffmpeg jobs                                           | `2`                  |

> AWS credentials are managed via IAM roles on EC2. No keys needed in environment variables.

## Response Envelope

All JSON responses follow the same envelope:

```json
{
  "success": true,
  "status": 200,
  "message": "ok",
  "data": {},
  "request_id": "0e8f...",
  "timestamp": "2026-04-25T12:00:00.000Z"
}
```

On failure, `success=false`, `data` is omitted and `error` carries the
machine-readable reason.

## Authentication

Admin endpoints (`/v1/a/*`) are not protected at the application layer. Restrict
access at the network/infrastructure layer (VPC, private load balancer, security
group, or IP allow-list) before exposing the service.

## Local Development

```sh
# Run with live reload
air

# Or standard
go run ./cmd/api
```

Server starts on `:8080`

## Quick Start

### Using Makefile (Recommended)

```sh
# View all available commands
make help

# Initialize backend and deploy infrastructure
make init-backend-dev
make infra-apply-dev

# Build and push Docker image
make docker-build
make docker-push

# Deploy application to EC2
make docker-deploy-dev EC2_IP=<your-ec2-ip>

# Setup SSL certificate
make ssl-setup-dev EC2_IP=<your-ec2-ip> DOMAIN=api.yourdomain.com
```

### Manual Deployment

```sh
# Initialize backend
./scripts/init-backend.sh bi8s dev us-east-1

# Deploy infrastructure
./scripts/deploy.sh dev apply

# On EC2: Configure and deploy
ssh ec2-user@<EC2_IP>
cd /opt/bi8s/compose
cp .env.example .env
vim .env  # Add your secrets
../scripts/deploy.sh
```

See [Deployment Guide](docs/deployment-guide.md) for complete instructions.

## API Endpoints

### Health

| Method | Path         | Description    |
| ------ | ------------ | -------------- |
| `GET`  | `/v1/health` | Liveness check |

### Content (Public Routes - /v1/c/)

| Method | Path                                   | Description                                  |
| ------ | -------------------------------------- | -------------------------------------------- |
| `GET`  | `/v1/c/content?type=all`               | Get recent content sorted by creation date   |
| `GET`  | `/v1/c/content/{contentId}`            | Get single content by ID                     |
| `GET`  | `/v1/c/people/{peopleId}`              | Get single person by ID                      |
| `GET`  | `/v1/c/people/{peopleId}/content`      | Get content by person ID                     |
| `GET`  | `/v1/c/banner?type=all`                | Get random banner                            |
| `GET`  | `/v1/c/attributes/{id}?content=all`    | Get content by attribute ID                  |
| `GET`  | `/v1/c/discover?type=latest`           | Discover content (latest, popular, trending) |
| `GET`  | `/v1/c/play/{contentType}/{contentId}` | Get playback information                     |

### Encoder

| Method | Path                    | Description                   |
| ------ | ----------------------- | ----------------------------- |
| `POST` | `/v1/c/encoder/new`     | Create new video encoding job |
| `GET`  | `/v1/c/encoder/{jobId}` | Get encoding job details      |

### Admin Routes (/v1/a/)

#### Content Management

| Method   | Path                        | Description             |
| -------- | --------------------------- | ----------------------- |
| `POST`   | `/v1/a/content/{contentId}` | Upload content assets   |
| `GET`    | `/v1/a/movies`              | List all movies (admin) |
| `GET`    | `/v1/a/movies/{movieId}`    | Get movie by ID (admin) |
| `POST`   | `/v1/a/movies`              | Create a movie          |
| `DELETE` | `/v1/a/movies/{movieId}`    | Delete a movie          |

#### People Management

| Method   | Path                              | Description                      |
| -------- | --------------------------------- | -------------------------------- |
| `GET`    | `/v1/a/people`                    | List all people                  |
| `GET`    | `/v1/a/people/{peopleId}`         | Get person by ID                 |
| `POST`   | `/v1/a/people`                    | Create a person                  |
| `DELETE` | `/v1/a/people/{peopleId}`         | Delete a person                  |
| `GET`    | `/v1/a/people/{peopleId}/content` | Get content by person ID (admin) |

#### Attributes Management

| Method   | Path                             | Description         |
| -------- | -------------------------------- | ------------------- |
| `GET`    | `/v1/a/attributes`               | List all attributes |
| `GET`    | `/v1/a/attributes/{attributeId}` | Get attribute by ID |
| `POST`   | `/v1/a/attributes`               | Create an attribute |
| `DELETE` | `/v1/a/attributes/{attributeId}` | Delete an attribute |

## Project Structure

```text
cmd/api/              # Application entry point
internal/             # Application code
  ├── http/           # HTTP handlers and routing
  ├── service/        # Business logic
  ├── repository/     # Data access layer
  ├── model/          # Domain models
  └── ...
infra/                # Infrastructure as Code
  ├── tofu/           # OpenTofu/Terraform configs
  ├── docker/         # Docker & Nginx configs
  └── scripts/        # Deployment scripts
docs/                 # Documentation
  ├── architecture.md
  ├── api.md
  └── ...
```

## Documentation

- [Architecture](docs/architecture.md) - System design and layers
- [API Reference](docs/api.md) - Complete API documentation
- [Configuration Guide](docs/configuration.md) - Project name and settings
- [Makefile Guide](docs/makefile-guide.md) - Using the Makefile for all operations
- [Deployment Guide](docs/deployment-guide.md) - Infrastructure and deployment
- [Deployment Workflow](docs/workflow.md) - Complete workflow with script usage
- [Deploy Scripts Comparison](docs/deploy-scripts-comparison.md) - Understanding the two deploy.sh scripts
- [Scripts Documentation](docs/scripts.md) - All scripts and their usage
- [DynamoDB Design](docs/dynamodb.md) - Database schema

## Author

[Vikas Singh](https://xanderbilla.com)
