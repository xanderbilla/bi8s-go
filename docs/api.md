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

Creates a new movie and uploads its poster to S3.

#### Create Movie Request Body

Content-Type: `multipart/form-data`

Form fields:

- `id` (optional string)
- `title` (required string)
- `description` (required string)
- `performer` (required string)
- `year` (required int)
- `poster` (required file: jpeg, png, webp, gif, avif)

If `id` is omitted, the service generates one.

Poster object naming convention:

- `movies/{movieId}/poster.{ext}`

```json
{
  "id": "generated-or-provided-id",
  "title": "The Dark Knight",
  "description": "Batman faces the Joker in Gotham.",
  "performer": "Christian Bale",
  "year": 2008,
  "poster": "movies/generated-or-provided-id/cover.jpg"
}
```

| Field         | Type   | Required | Description                                         |
| ------------- | ------ | -------- | --------------------------------------------------- |
| `id`          | string | no       | Unique ID for the movie (auto-generated if missing) |
| `title`       | string | yes      | Title of the movie                                  |
| `description` | string | yes      | Short movie description                             |
| `performer`   | string | yes      | Main performer name                                 |
| `year`        | int    | yes      | Year the movie was released                         |
| `poster`      | file   | yes      | Poster image uploaded to S3                         |

Validation rules:

- `id`: optional, 1-64 chars when provided
- `title`: required, 1-255 chars
- `description`: required, 1-255 chars
- `performer`: required, 1-128 chars
- `year`: required, 1888-2100
- `poster`: required, max 10MB, image types only

#### Create Movie Response

```json
{
  "status": 201,
  "message": "movie created",
  "data": { "id": "3", "title": "The Dark Knight", "year": 2008 }
}
```

Returns `400 Bad Request` if the body is malformed.
Returns `400 Bad Request` for invalid multipart forms, invalid file types, or oversized poster files.
Returns `400 Bad Request` for validation errors.
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
