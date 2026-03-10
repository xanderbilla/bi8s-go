# Architecture

This project follows a layered architecture. Each layer has one job and talks only to the layer directly below it. This keeps the code easy to read, test, and change.

## Layers

```
HTTP Request
     |
     v
  Handler        (internal/http/)
     |
     v
  Service        (internal/service/)
     |
     v
  Repository     (internal/repository/)
     |
     v
  DynamoDB       (AWS)
```

### Handler

The handler is the first thing that runs when an HTTP request comes in. Its only job is to:

- Read the request (URL params, request body)
- Call the right service method
- Write the response (success or error)

Handlers do not talk to the database directly and do not contain business logic.

### Service

The service sits between the handler and the database. This is where business logic lives — things like validation, transformations, or rules like "a movie must have a title".

Right now the service methods are mostly pass-throughs, but that is intentional. As the app grows, you add logic here without touching the handler or the repository.

### Repository

The repository is the only layer that talks to DynamoDB. It contains all the read and write operations. If you ever switch from DynamoDB to another database, only this file changes — nothing else in the app needs to know.

The repository is defined as a Go interface (`MovieRepository`), which means you can easily swap it out or mock it in tests.

## Application Container

The `app.Application` struct is created once at startup and passed to every handler. It holds the config, the AWS clients, and the services. This avoids using global variables, which makes the code easier to test and reason about.

## Request Flow (example: POST /v1/movies)

1. Request arrives at the router
2. Chi middleware runs (request ID, logger, recoverer, timeout)
3. `MovieHandler.CreateMovie` reads the body and validates the shape
4. Handler calls `MovieService.Create`
5. Service calls `DynamoMovieRepository.Create`
6. Repository writes to DynamoDB
7. Handler sends a `201 Created` JSON response back to the client
