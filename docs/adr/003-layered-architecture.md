# ADR-003: Use a layered architecture (Handler, Service, Repository)

## Decision

Split the code into three layers: Handler, Service, and Repository. Each layer only talks to the one directly below it.

## Why

This is a common pattern in Go backends and it solves a real problem: without it, handler code ends up doing database queries directly, which makes the code hard to test and hard to change.

With this separation:

- Handlers stay thin — they only deal with HTTP (reading requests, writing responses).
- The service layer is where you add business rules later without touching handlers or the database.
- The repository layer is the only place that knows about DynamoDB. Swapping the database means changing one file.

It is a small amount of extra structure upfront, but it pays off quickly as the app grows.
