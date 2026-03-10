# ADR-002: Use DynamoDB as the database

## Decision

Use AWS DynamoDB as the database instead of a relational database like PostgreSQL or MySQL.

## Why

The project runs on AWS and DynamoDB is the simplest serverless database option there. It requires no server to manage, scales automatically, and has a generous free tier.

For a simple key-value access pattern (look up a movie by ID), DynamoDB is a good fit. The trade-off is that complex queries (filtering by title, sorting by year) are harder than they would be in SQL.

The repository layer is written behind a Go interface (`MovieRepository`), so if the app outgrows DynamoDB, the database can be swapped out without changing the handler or service code.
