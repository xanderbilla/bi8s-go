# ADR-001: Use chi as the HTTP router

## Decision

Use [chi](https://github.com/go-chi/chi) as the HTTP router instead of Go's built-in `net/http` mux or a larger framework like Gin.

## Why

Go's built-in mux does not support URL parameters like `{movieId}`, which are needed for routes like `GET /v1/movies/{id}`. Chi adds this without pulling in a large dependency.

Chi is also idiomatic Go — middleware is just a function, handlers follow the standard `http.Handler` interface, and there is no "framework magic". This means the code is easy to read and you are not locked in to any non-standard patterns.

Larger frameworks like Gin or Fiber are faster in benchmarks but add complexity and break away from the standard library. For a project this size, the difference is irrelevant.
