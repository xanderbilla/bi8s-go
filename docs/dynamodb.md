# DynamoDB

This document explains how DynamoDB is used in this project, the current table design, and known limitations.

## What is DynamoDB?

DynamoDB is a NoSQL database provided by AWS. Unlike a traditional relational database (like PostgreSQL or MySQL), DynamoDB does not have tables with rows and columns in the usual sense. Instead, it stores items (think of them as JSON objects), and you look them up using keys.

The main things to know:

- Every item must have a **partition key** (the primary way to look something up).
- DynamoDB is extremely fast at looking up a single item by its partition key.
- Looking up many items at once (without a key) requires a **Scan**, which reads the whole table.

## Table Design

This project uses 4 DynamoDB tables:

### 1. Movies/Content Table

**Name:** `bi8s-content-table-{env}` (e.g., `bi8s-content-table-dev`)

| Attribute | Type   | Role          |
| --------- | ------ | ------------- |
| `id`      | String | Partition key |
| `title`   | String | Regular field |
| `year`    | Number | Regular field |
| ...       | ...    | Other fields  |

### 2. Persons Table

**Name:** `bi8s-person-table-{env}` (e.g., `bi8s-person-table-dev`)

| Attribute | Type   | Role          |
| --------- | ------ | ------------- |
| `id`      | String | Partition key |
| `name`    | String | Regular field |
| ...       | ...    | Other fields  |

### 3. Attributes Table

**Name:** `bi8s-attributes-table-{env}` (e.g., `bi8s-attributes-table-dev`)

| Attribute | Type   | Role          |
| --------- | ------ | ------------- |
| `id`      | String | Partition key |
| `name`    | String | Regular field |
| `type`    | String | Regular field |

### 4. Video Encoder Table

**Name:** `bi8s-video-table-{env}` (e.g., `bi8s-video-table-dev`)

| Attribute | Type   | Role          |
| --------- | ------ | ------------- |
| `id`      | String | Partition key |
| ...       | ...    | Other fields  |

All tables use a simple partition key design with no sort key. Each item is looked up directly by its `id`.

## How the Code Talks to DynamoDB

All DynamoDB operations live in `internal/repository/*_dynamo.go`. No other layer knows or cares that DynamoDB is being used.

| Operation | DynamoDB API call | Notes                                                                   |
| --------- | ----------------- | ----------------------------------------------------------------------- |
| Get all   | `Scan`            | Reads the whole table — see limitations below                           |
| Get one   | `GetItem`         | Uses `ConsistentRead: true` to always return the latest data            |
| Create    | `PutItem`         | Uses `attribute_not_exists(id)` to prevent overwriting an existing item |
| Delete    | `DeleteItem`      | Uses `attribute_exists(id)` to fail if the item does not exist          |

## Consistent Reads

When reading a single item (`GetItem`), the code uses `ConsistentRead: true`. This means DynamoDB will always return the most up-to-date version of the item, even if it was just written a millisecond ago. Without this, DynamoDB might return slightly stale data from a replica.

Consistent reads cost twice as many read capacity units, but for a small app this is not a concern.

## AWS Credentials

The application uses **IAM roles** when running on EC2. No AWS access keys are needed in environment variables. The AWS SDK automatically uses the instance profile credentials.

## Known Limitations

### Scan is not ideal for large tables

`GetAll` uses a `Scan` operation, which reads every item in the table from start to finish. This works fine when the table is small, but has two problems at scale:

1. DynamoDB returns at most 1MB of data per Scan call. If the table is larger than 1MB, you only get the first page back.

2. Scan reads every single item in the table, even if you only need a few. This wastes read capacity and costs money.

The solution is to add pagination support using `ExclusiveStartKey` and `LastEvaluatedKey`. See [todo.md](todo.md) for details.

### No secondary indexes

Right now you can only look up items by their `id`. If you want to query movies by title or year, you would need to add a Global Secondary Index (GSI).

## Table Configuration

Tables are created and managed by Terraform/OpenTofu. See `infra/tofu/modules/dynamodb/` for the infrastructure code.

Configuration:

- **Billing Mode:** PAY_PER_REQUEST (on-demand)
- **Encryption:** Enabled (server-side)
- **Point-in-time Recovery:** Enabled
- **Backups:** Managed by AWS

## Environment Variables

Table names are set via environment variables:

- `DYNAMODB_MOVIE_TABLE` - Movies/content table
- `DYNAMODB_PERSON_TABLE` - Persons table
- `DYNAMODB_ATTRIBUTE_TABLE` - Attributes table
- `DYNAMODB_ENCODER_TABLE` - Video encoder table

These are automatically configured by Terraform during deployment.
