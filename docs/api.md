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

**Response**

```json
{
  "status": 200,
  "message": "ok",
  "data": {
    "status": "ok",
    "env": "dev"
  }
}
```

## Movies

### GET /v1/movies

Returns all movies in the database.

**Response**

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

**URL Parameters**

| Parameter | Type   | Description         |
| --------- | ------ | ------------------- |
| `id`      | string | The ID of the movie |

**Response**

```json
{
  "status": 200,
  "message": "movie fetched",
  "data": { "id": "1", "title": "Inception", "year": 2010 }
}
```

Returns `null` for `data` if no movie with that ID exists.

### POST /v1/movies

Creates a new movie.

**Request Body**

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

**Response**

```json
{
  "status": 201,
  "message": "movie created",
  "data": { "id": "3", "title": "The Dark Knight", "year": 2008 }
}
```

Returns `400 Bad Request` if the body is malformed.
Returns `500 Internal Server Error` if a movie with the same ID already exists.

### DELETE /v1/movies/{id}

Deletes a movie by its ID.

**URL Parameters**

| Parameter | Type   | Description         |
| --------- | ------ | ------------------- |
| `id`      | string | The ID of the movie |

**Response**

```json
{
  "status": 200,
  "message": "movie deleted",
  "data": null
}
```

Returns `500 Internal Server Error` if no movie with that ID exists.
