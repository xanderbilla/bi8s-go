# Performance

This document explains what the benchmark numbers mean, why the latency is what it is,
and how to improve it.

## Benchmark Results (local dev machine)

These numbers were measured by sending 200 requests with 20 concurrent connections
using Apache Bench (`ab`), while running the server locally and talking to a real
DynamoDB table in AWS.

| Endpoint           | Requests/sec | Avg latency | What it does               |
| ------------------ | ------------ | ----------- | -------------------------- |
| `GET /v1/health`   | ~15,000      | 1 ms        | No database, pure memory   |
| `GET /v1/movies/1` | ~56          | 357 ms      | One DynamoDB `GetItem`     |
| `GET /v1/movies`   | ~47          | 425 ms      | DynamoDB `Scan` (all rows) |

## Why is the DB latency 300-400ms?

The short answer: the request has to travel from your laptop to AWS and back over the
public internet, and that round trip takes time.

Here is the path every request takes:

```
Your laptop
    |
    |  ~150-200ms (public internet, round trip to AWS)
    v
AWS us-east-1
    |
    |  ~1-5ms (internal AWS network)
    v
DynamoDB
    |
    |  ~1-5ms (internal AWS network, back to app)
    v
Your laptop
```

The Go server itself handles each request in under 1ms. The other 350ms is just
waiting for the network. This is completely normal when developing locally against
a cloud database.

## Is 300-400ms bad?

For a production app, yes. Users generally expect API responses under 100ms,
and most well-built APIs respond in under 50ms.

For local development, it is expected and not something you need to fix right now.

## How to bring it down in production

The improvements are listed from highest impact to lowest.

### 1. Deploy the app on AWS in the same region as DynamoDB

This is by far the biggest win. When the app runs on AWS (e.g. on EC2, ECS, or Lambda)
in the same region as DynamoDB, the network hop is replaced by AWS's private internal
network, which is extremely fast.

Expected result: 300-400ms drops to 1-5ms, with no code changes at all.

### 2. Add an in-memory cache

Some data barely ever changes — for example, a movie created in 2010 is not going to
update its title tomorrow. For data like this, you can store the result in memory the
first time you fetch it, and then return it from memory on the next request without
hitting DynamoDB at all.

In Go, the simplest version is a `sync.Map` with a TTL (time-to-live) — after a set
amount of time, the cached value is considered stale and the next request refreshes it.
For something more serious, Redis is a popular choice.

Expected result: Cached reads drop to under 1ms.

### 3. Enable DAX (DynamoDB Accelerator)

DAX is an AWS-managed in-memory cache that sits directly in front of DynamoDB.
You point your app at DAX instead of DynamoDB, and DAX handles caching automatically.
It is designed specifically for DynamoDB and can serve reads in microseconds.

The trade-off is cost — DAX is not free and is overkill for small apps.

### 4. Use VPC endpoints for DynamoDB

When running on AWS, a VPC endpoint means your traffic between the app and DynamoDB
never leaves AWS's private network, even if they are in the same region. This removes
any chance of traffic going through the public internet and slightly reduces latency.

This is a small improvement on top of option 1, not a replacement for it.

## Summary

The current latency is not a code problem — it is a deployment problem.
The Go server is fast. Once the app is deployed on AWS in the same region as DynamoDB,
the latency will drop dramatically. Caching can then be added on top for hot data.
