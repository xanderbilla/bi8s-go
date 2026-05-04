# Database (DynamoDB)

`bi8s-go` is fully serverless on the data side: every domain object
lives in DynamoDB, accessed through the repositories under
`app/internal/repository/`. There is no relational store.

## Tables

Table names below are for the `dev` environment. Production names
substitute `-prod`. Names are configurable via `DYNAMODB_*_TABLE`.

### `bi8s-content-table-dev` — Movies / content

| Attribute                 | Type | Notes                                             |
| ------------------------- | ---- | ------------------------------------------------- |
| `contentId`               | S    | **Partition key**. Stable, externally visible id. |
| `type`                    | S    | `movie`, `series`, `episode`, ...                 |
| `title`                   | S    |                                                   |
| `attributes`              | L<S> | Attribute ids referenced by the item.             |
| `people`                  | L<M> | Cast / crew with role.                            |
| `assets`                  | M    | Poster, trailer, source, HLS manifest URLs.       |
| `playback`                | M    | DRM metadata, manifest URL, available qualities.  |
| `createdAt` / `updatedAt` | S    | RFC 3339.                                         |

Access patterns:

| Pattern                    | Operation                                               |
| -------------------------- | ------------------------------------------------------- |
| Get by id                  | `GetItem(contentId)`                                    |
| List all (admin)           | `Scan` (paginated, capped by `DYNAMODB_MAX_SCAN_PAGES`) |
| Recent / banner / discover | `Scan` + in-process filter (small catalog)              |

### `bi8s-person-table-dev` — People

| Attribute                 | Type | Notes                    |
| ------------------------- | ---- | ------------------------ |
| `personId`                | S    | **Partition key**.       |
| `name`                    | S    |                          |
| `roles`                   | L<S> | `actor`, `director`, ... |
| `images`                  | M    | Profile + headshot keys. |
| `createdAt` / `updatedAt` | S    |                          |

Access patterns:

| Pattern                      | Operation                                                         |
| ---------------------------- | ----------------------------------------------------------------- |
| Get by id                    | `GetItem(personId)`                                               |
| List all                     | `Scan`                                                            |
| Content credited to a person | join via `bi8s-content-table-dev` (filter by `people[].personId`) |

### `bi8s-attributes-table-dev` — Attributes (genres, tags)

| Attribute     | Type | Notes                                  |
| ------------- | ---- | -------------------------------------- |
| `attributeId` | S    | **Partition key**.                     |
| `name`        | S    | **`name-index` GSI** (lookup by name). |
| `kind`        | S    | `genre`, `mood`, `language`, ...       |
| `description` | S    |                                        |

GSI: `name-index` — partition key `name`. Configurable via
`DYNAMODB_ATTRIBUTE_NAME_INDEX` (default `name-index`).

Access patterns:

| Pattern     | Operation               |
| ----------- | ----------------------- |
| Get by id   | `GetItem(attributeId)`  |
| Get by name | `Query` on `name-index` |
| List all    | `Scan`                  |

### `bi8s-video-table-dev` — Encoder jobs

| Attribute                 | Type | Notes                                                     |
| ------------------------- | ---- | --------------------------------------------------------- |
| `jobId`                   | S    | **Partition key**.                                        |
| `contentId`               | S    | **`contentId-index` GSI** (find jobs for a content item). |
| `status`                  | S    | `queued`, `running`, `succeeded`, `failed`.               |
| `inputKey`                | S    | Source S3 key.                                            |
| `outputPrefix`            | S    | HLS output prefix in S3.                                  |
| `progress`                | N    | 0–100.                                                    |
| `error`                   | S    | Populated on failure.                                     |
| `createdAt` / `updatedAt` | S    |                                                           |

GSI: `contentId-index` — partition key `contentId`. Required at
startup via `DYNAMODB_ENCODER_CONTENT_ID_INDEX`.

## Schema rules

- **Strings are UTF-8.** No client-side encoding tricks.
- **Timestamps are RFC 3339** (e.g. `2025-05-04T04:30:00Z`).
- **No `null` attributes.** Optional fields are simply omitted on writes.
- **All ids are server-generated** (`uuid.NewString()`); clients never
  supply them.

## Capacity & limits

| Setting      | Dev                     | Prod                                |
| ------------ | ----------------------- | ----------------------------------- |
| Billing mode | Provisioned + autoscale | On-demand                           |
| PITR         | enabled                 | enabled                             |
| TTL          | none                    | none                                |
| Streams      | disabled                | disabled (enable for CDC if needed) |

`DYNAMODB_MAX_SCAN_PAGES` (default 1000) caps long-running scans so a
runaway request can't exhaust capacity.

## Local DynamoDB

For Compose-only flows the API reads from real AWS by default. To use
DynamoDB Local instead:

```bash
# add a dynamodb-local service to docker-compose.local.yml and set
AWS_ENDPOINT_URL=http://dynamodb-local:8000
# create tables with the AWS CLI:
aws dynamodb create-table \
    --endpoint-url http://localhost:8000 \
    --table-name bi8s-content-table-dev \
    --attribute-definitions AttributeName=contentId,AttributeType=S \
    --key-schema AttributeName=contentId,KeyType=HASH \
    --billing-mode PAY_PER_REQUEST
# (repeat for the other three tables + their GSIs)
```

## Migrations

There is no schema-migration tool — DynamoDB is schemaless. To evolve a
field shape:

1. Write code that **reads both old and new shapes**.
2. Deploy.
3. Run a one-off backfill (`scripts/seed.sh`-style) to rewrite items.
4. Remove the old-shape read path in a follow-up release.

GSI changes (add/drop) are made via Tofu (`infra/tofu/modules/dynamodb`).
DynamoDB applies them online; no downtime is needed.
