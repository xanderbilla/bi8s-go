# DynamoDB

This document explains how DynamoDB is used in this project, the current table design, and known limitations.

## What is DynamoDB?

DynamoDB is a NoSQL database provided by AWS. Unlike a traditional relational database (like PostgreSQL or MySQL), DynamoDB does not have tables with rows and columns in the usual sense. Instead, it stores items (think of them as JSON objects), and you look them up using keys.

The main things to know:

- Every item must have a **partition key** (the primary way to look something up).
- DynamoDB is extremely fast at looking up a single item by its partition key.
- Looking up many items at once (without a key) requires a **Scan**, which reads the whole table.

## Table Design

Table name: `bi8s-dev` (configurable via the `DYNAMODB_TABLE` environment variable)

| Attribute | Type   | Role          |
| --------- | ------ | ------------- |
| `id`      | String | Partition key |
| `title`   | String | Regular field |
| `year`    | Number | Regular field |

There is no sort key. Each movie is looked up directly by its `id`.

## How the Code Talks to DynamoDB

All DynamoDB operations live in `internal/repository/movies_dynamo.go`. No other layer knows or cares that DynamoDB is being used.

| Operation | DynamoDB API call | Notes                                                                   |
| --------- | ----------------- | ----------------------------------------------------------------------- |
| Get all   | `Scan`            | Reads the whole table — see limitations below                           |
| Get one   | `GetItem`         | Uses `ConsistentRead: true` to always return the latest data            |
| Create    | `PutItem`         | Uses `attribute_not_exists(id)` to prevent overwriting an existing item |
| Delete    | `DeleteItem`      | Uses `attribute_exists(id)` to fail if the item does not exist          |

## Consistent Reads

When reading a single movie (`GetItem`), the code uses `ConsistentRead: true`. This means DynamoDB will always return the most up-to-date version of the item, even if it was just written a millisecond ago. Without this, DynamoDB might return slightly stale data from a replica.

Consistent reads cost twice as many read capacity units, but for a small app this is not a concern.

## Known Limitations

### Scan is not ideal for large tables

`GetAll` uses a `Scan` operation, which reads every item in the table from start to finish. This works fine when the table is small, but has two problems at scale:

1. DynamoDB returns at most 1MB of data per Scan call. If the table is larger than 1MB, you only get the first page back.
2. Scan consumes read capacity proportional to the total size of the table, which becomes expensive.

The fix is to use pagination with `ExclusiveStartKey` and `LastEvaluatedKey`, or to redesign the access pattern to use a `Query` instead of a `Scan`.

This is tracked in [todo.md](todo.md).

## Running Locally

You do not need a real AWS account to develop locally. You can run a local version of DynamoDB using Docker:

```sh
docker run -p 8000:8000 amazon/dynamodb-local
```

Then point the app at it by setting `AWS_ENDPOINT_URL=http://localhost:8000` (requires adding endpoint override support to `internal/aws/config.go`).
