# API Reference

Base URL: `http://localhost:8080`

All responses follow the same envelope shape:

```json
{
  "status": 200,
  "message": "some message",
  "data": { ... }
}
```

On errors:

```json
{
  "status": 400,
  "message": "request failed",
  "error": "description of what went wrong"
}
```

## Health

### GET /v1/health

Returns the current runtime environment. Use this to check the server is running.

#### Health Response

```json
{
  "status": 200,
  "message": "Health check passed!",
  "data": {
    "version": "dev"
  }
}
```

## Movies

### GET /v1/movies

Returns all movies in the database.

#### List Movies Response

```json
{
  "status": 200,
  "message": "movies fetched",
  "data": [
    { "id": "1", "title": "Inception", "year": 2010 },
    { "id": "2", "title": "Interstellar", "year": 2014 }
  ]
}
```

Returns `null` for `data` if there are no movies yet.

### GET /v1/movies/{id}

Returns a single movie by its ID.

#### Get Movie URL Parameters

| Parameter | Type   | Description         |
| --------- | ------ | ------------------- |
| `id`      | string | The ID of the movie |

#### Get Movie Response

```json
{
  "status": 200,
  "message": "movie fetched",
  "data": { "id": "1", "title": "Inception", "year": 2010 }
}
```

If no movie exists with that ID:

```json
{
  "status": 404,
  "message": "request failed",
  "error": "The requested resource was not found"
}
```

### POST /v1/movies

Creates a new movie.

#### Create Movie Request Body

```json
{
  "id": "3",
  "title": "The Dark Knight",
  "year": 2008
}
```

| Field   | Type   | Required | Description                 |
| ------- | ------ | -------- | --------------------------- |
| `id`    | string | yes      | Unique ID for the movie     |
| `title` | string | yes      | Title of the movie          |
| `year`  | int    | yes      | Year the movie was released |

Validation rules:

- `id`: required, 1-64 chars
- `title`: required, 1-255 chars
- `year`: required, 1888-2100

#### Create Movie Response

```json
{
  "status": 201,
  "message": "movie created",
  "data": { "id": "3", "title": "The Dark Knight", "year": 2008 }
}
```

Returns `400 Bad Request` if the body is malformed.
Returns `400 Bad Request` for validation errors (including unknown fields and multiple JSON objects in a single body).
Returns `409 Conflict` if a movie with the same ID already exists.

### DELETE /v1/movies/{id}

Deletes a movie by its ID.

#### Delete Movie URL Parameters

| Parameter | Type   | Description         |
| --------- | ------ | ------------------- |
| `id`      | string | The ID of the movie |

#### Delete Movie Response

```json
{
  "status": 200,
  "message": "movie deleted"
}
```

If no movie exists with that ID:

```json
{
  "status": 404,
  "message": "request failed",
  "error": "The requested resource was not found"
}
```

## Router-Level Errors

The API also returns JSON for unmatched routes and unsupported methods.

- Unknown route: `404 Not Found`
- Unsupported method on a valid route: `405 Method Not Allowed`
