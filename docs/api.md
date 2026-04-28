# API Reference

Base URL: `http://localhost:8080`

All responses follow the same envelope shape:

```json
{
  "success": true,
  "status": 200,
  "message": "some message",
  "data": {},
  "request_id": "<uuid>",
  "timestamp": "2026-04-25T12:00:00.000Z"
}
```

On errors:

```json
{
  "success": false,
  "status": 400,
  "message": "description of what went wrong",
  "request_id": "<uuid>",
  "timestamp": "2026-04-25T12:00:00.000Z"
}
```

### Authentication

Admin routes (`/v1/a/*`) are not authenticated at the application layer. Restrict
access via network controls (private VPC, internal load balancer, IP allow-list)
before exposing the service.

## API Structure

The API is organized into two main sections:

- `/v1/c/` - Content routes (public-facing, filtered content)
- `/v1/a/` - Admin routes (full access, all fields including stats and audit)

## Health

### GET /v1/health

Returns the current runtime environment. Use this to check the server is running.

**Response:**

```json
{
  "status": 200,
  "message": "Health check passed!",
  "data": {
    "version": "prod"
  }
}
```

## Content Routes (Public - /v1/c/)

### GET /v1/c/content

Returns recent content sorted by creation date. Only returns content with visibility=PUBLIC and status=RELEASED or IN_PRODUCTION.

**Query Parameters:**

| Parameter | Type   | Required | Description                                             |
| --------- | ------ | -------- | ------------------------------------------------------- |
| `type`    | string | no       | Filter by contentType (movie, tv, all). Defaults to all |

**Response:**

```json
{
  "status": 200,
  "message": "movies fetched",
  "data": [
    {
      "id": "deb1f6e5-cc3d-4e55-80f6-3043a1b98692",
      "title": "Inception",
      "backdropPath": "movies/deb1f6e5-cc3d-4e55-80f6-3043a1b98692/cover.jpg",
      "posterPath": "movies/deb1f6e5-cc3d-4e55-80f6-3043a1b98692/poster.jpg",
      "releaseDate": "2010-07-16",
      "tags": [{ "id": "624874", "name": "Mind Bending" }],
      "contentRating": "18_PLUS",
      "contentType": "MOVIE"
    }
  ]
}
```

### GET /v1/c/content/{contentId}

Returns a single content by ID. Only returns content with visibility=PUBLIC and status=RELEASED or IN_PRODUCTION.

**URL Parameters:**

| Parameter   | Type   | Description           |
| ----------- | ------ | --------------------- |
| `contentId` | string | The ID of the content |

**Response:**

```json
{
  "status": 200,
  "message": "movie fetched",
  "data": {
    "id": "deb1f6e5-cc3d-4e55-80f6-3043a1b98692",
    "title": "Inception",
    "overview": "A thief who steals corporate secrets...",
    "backdropPath": "movies/deb1f6e5-cc3d-4e55-80f6-3043a1b98692/cover.jpg",
    "posterPath": "movies/deb1f6e5-cc3d-4e55-80f6-3043a1b98692/poster.jpg",
    "releaseDate": "2010-07-16",
    "firstAirDate": "2010-07-16",
    "contentRating": "18_PLUS",
    "originalLanguage": "en",
    "genres": [{ "id": "624871", "name": "Action" }],
    "casts": [{ "id": "231931", "name": "Leonardo DiCaprio" }],
    "tags": [{ "id": "624874", "name": "Mind Bending" }],
    "contentType": "MOVIE",
    "originCountry": ["US", "UK"],
    "moodTags": [{ "id": "624877", "name": "Intense" }],
    "runtime": 148,
    "status": "RELEASED",
    "tagline": "Your mind is the scene of the crime",
    "studios": [{ "id": "624870", "name": "Warner Bros. Pictures" }]
  }
}
```

**Error Response (404):**

```json
{
  "status": 404,
  "message": "The requested resource was not found"
}
```

### GET /v1/c/people/{peopleId}

Returns a single person by ID.

**URL Parameters:**

| Parameter  | Type   | Description          |
| ---------- | ------ | -------------------- |
| `peopleId` | string | The ID of the person |

### GET /v1/c/people/{peopleId}/content

Returns all content where the person is in the cast. Only returns content with visibility=PUBLIC and status=RELEASED or IN_PRODUCTION.

**URL Parameters:**

| Parameter  | Type   | Required | Description          |
| ---------- | ------ | -------- | -------------------- |
| `peopleId` | string | yes      | The ID of the person |

**Query Parameters:**

| Parameter | Type   | Required | Description                            |
| --------- | ------ | -------- | -------------------------------------- |
| `type`    | string | no       | Filter by contentType (movie, tv, all) |

### GET /v1/c/banner

Returns a random banner content for display. Only returns content with visibility=PUBLIC and status=RELEASED or IN_PRODUCTION.

**Query Parameters:**

| Parameter | Type   | Required | Description                            |
| --------- | ------ | -------- | -------------------------------------- |
| `type`    | string | no       | Filter by contentType (movie, tv, all) |

### GET /v1/c/attributes/{id}

Returns all content that have the specified attribute (genre, tag, or mood tag). Only returns content with visibility=PUBLIC and status=RELEASED or IN_PRODUCTION.

**URL Parameters:**

| Parameter | Type   | Description                                   |
| --------- | ------ | --------------------------------------------- |
| `id`      | string | The ID of the attribute (genre, tag, or mood) |

**Query Parameters:**

| Parameter | Type   | Required | Description                            |
| --------- | ------ | -------- | -------------------------------------- |
| `content` | string | no       | Filter by contentType (movie, tv, all) |

### GET /v1/c/discover

Discover content by type (latest, popular, trending). Only returns content with visibility=PUBLIC and status=RELEASED or IN_PRODUCTION.

**Query Parameters:**

| Parameter | Type   | Required | Description                                             |
| --------- | ------ | -------- | ------------------------------------------------------- |
| `type`    | string | yes      | Discovery type (latest, popular, trending)              |
| `content` | string | no       | Filter by contentType (movie, tv, all). Defaults to all |

### GET /v1/c/play/{contentType}/{contentId}

Get playback information for a specific content.

**URL Parameters:**

| Parameter     | Type   | Description                 |
| ------------- | ------ | --------------------------- |
| `contentType` | string | Type of content (movie, tv) |
| `contentId`   | string | The ID of the content       |

## Encoder Routes

### POST /v1/c/encoder/new

Create a new video encoding job.

### GET /v1/c/encoder/{jobId}

Get encoding job details.

**URL Parameters:**

| Parameter | Type   | Description       |
| --------- | ------ | ----------------- |
| `jobId`   | string | The ID of the job |

## Admin Routes (/v1/a/)

Admin routes provide full access to all fields including stats and audit information, without filtering by visibility or status.

### POST /v1/a/content/{contentId}

Upload content assets (video files, images, etc.).

**URL Parameters:**

| Parameter   | Type   | Description           |
| ----------- | ------ | --------------------- |
| `contentId` | string | The ID of the content |

### GET /v1/a/movies

Returns all movies (admin view with all fields including stats and audit).

### GET /v1/a/movies/{movieId}

Returns a single movie by ID (admin view with all fields).

**URL Parameters:**

| Parameter | Type   | Description         |
| --------- | ------ | ------------------- |
| `movieId` | string | The ID of the movie |

### POST /v1/a/movies

Creates a new movie with poster and cover image uploads.

**Content-Type:** `multipart/form-data`

**Form Fields:**

| Field               | Type   | Required | Description                                                     |
| ------------------- | ------ | -------- | --------------------------------------------------------------- |
| `title`             | string | yes      | Movie title (1-128 chars)                                       |
| `overview`          | string | yes      | Movie description (1-1000 chars)                                |
| `release_date`      | string | no       | Release date in YYYY-MM-DD format (2006-01-01 to today)         |
| `first_air_date`    | string | no       | First air date in YYYY-MM-DD format (2006-01-01 to today)       |
| `adult`             | bool   | no       | Adult content flag                                              |
| `content_rating`    | string | no       | Content rating: 18_PLUS or 21_PLUS                              |
| `original_language` | string | yes      | Language code: en, hi, ja, ko, fr, es                           |
| `genres`            | string | no       | Comma-separated genre names (auto-generates IDs)                |
| `casts`             | string | no       | Comma-separated performer IDs (names fetched from person table) |
| `tags`              | string | no       | Comma-separated tag names (auto-generates IDs)                  |
| `content_type`      | string | no       | Content type: MOVIE or TV                                       |
| `origin_country`    | string | no       | Comma-separated country codes                                   |
| `mood_tags`         | string | no       | Comma-separated mood tag names (auto-generates IDs)             |
| `runtime`           | int    | no       | Runtime in minutes                                              |
| `status`            | string | no       | Status: RELEASED, IN_PRODUCTION, etc.                           |
| `tagline`           | string | no       | Movie tagline (max 255 chars)                                   |
| `studios`           | string | no       | Comma-separated studio names (auto-generates IDs)               |
| `visibility`        | string | no       | Visibility: PUBLIC or PRIVATE (defaults to PUBLIC)              |
| `poster`            | file   | no       | Poster image (max 10MB, jpeg/png/webp/gif/avif)                 |
| `cover`             | file   | no       | Backdrop/cover image (max 10MB, jpeg/png/webp/gif/avif)         |

**Cast Validation:**

- Only performer IDs are accepted (not names)
- Performer must exist in person table
- Names are automatically fetched from person table

**Example Request:**

```bash
curl -X POST http://localhost:8080/v1/movies \
  -F "title=Inception" \
  -F "overview=A thief who steals corporate secrets..." \
  -F "release_date=2010-07-16" \
  -F "content_rating=18_PLUS" \
  -F "original_language=en" \
  -F "genres=Action,Science Fiction,Thriller" \
  -F "casts=231931" \
  -F "tags=Mind Bending,Heist,Dreams" \
  -F "content_type=MOVIE" \
  -F "runtime=148" \
  -F "status=RELEASED" \
  -F "tagline=Your mind is the scene of the crime" \
  -F "studios=Warner Bros. Pictures" \
  -F "poster=@poster.jpg" \
  -F "cover=@cover.jpg"
```

**Response (201):**

```json
{
  "status": 201,
  "message": "movie created",
  "data": {
    "id": "deb1f6e5-cc3d-4e55-80f6-3043a1b98692",
    "title": "Inception",
    "casts": [{ "id": "231931", "name": "Leonardo DiCaprio" }],
    "stats": {
      "totalViews": 0,
      "totalLikes": 0,
      "averageRating": 0
    },
    "audit": {
      "createdAt": "2026-03-31T17:20:53.657594+05:30",
      "version": 1,
      "isDeleted": false
    }
  }
}
```

**Error Responses:**

```json
// 400 - Performer not found
{
  "status": 400,
  "message": "performer with id '999999' not found"
}

// 400 - Invalid date
{
  "status": 400,
  "message": "Key: 'Movie.ReleaseDate' Error:Field validation for 'ReleaseDate' failed on the 'daterange' tag"
}

// 409 - Duplicate ID
{
  "status": 409,
  "message": "The resource already exists"
}
```

### DELETE /v1/a/movies/{movieId}

Deletes a movie by ID.

**URL Parameters:**

| Parameter | Type   | Description         |
| --------- | ------ | ------------------- |
| `movieId` | string | The ID of the movie |

**Response (200):**

```json
{
  "status": 200,
  "message": "movie deleted"
}
```

**Error Response (404):**

```json
{
  "status": 404,
  "message": "The requested resource was not found"
}
```

### GET /v1/a/people

Returns all people (admin view with all fields).

**Response:**

```json
{
  "status": 200,
  "message": "persons fetched",
  "data": [
    {
      "id": "231931",
      "contentType": "PERSON",
      "name": "Leonardo DiCaprio",
      "roles": ["PERFORMER"],
      "verified": false,
      "active": true,
      "careerStatus": "Active"
    }
  ]
}
```

### GET /v1/a/people/{peopleId}

Returns a single person by ID (admin view with all fields).

**URL Parameters:**

| Parameter  | Type   | Description          |
| ---------- | ------ | -------------------- |
| `peopleId` | string | The ID of the person |

**Response:**

```json
{
  "status": 200,
  "message": "person fetched",
  "data": {
    "id": "231931",
    "contentType": "PERSON",
    "name": "Leonardo DiCaprio",
    "roles": ["PERFORMER"],
    "stageName": "Leonardo DiCaprio",
    "bio": "Academy Award-winning actor...",
    "birthDate": "1974-11-11",
    "birthPlace": "Los Angeles, California, USA",
    "nationality": "American",
    "gender": "Male",
    "height": 183,
    "verified": false,
    "active": true,
    "debutYear": 1991,
    "careerStatus": "Active",
    "profilePath": "person/231931/profile.jpg",
    "backdropPath": "person/231931/cover.jpg",
    "stats": {
      "totalProductions": 0,
      "totalViews": 0,
      "subscriberCount": 0,
      "followersCount": 0,
      "averageRating": 0
    },
    "audit": {
      "createdAt": "2026-03-31T17:20:13.193213+05:30",
      "version": 1,
      "isDeleted": false
    }
  }
}
```

### POST /v1/a/people

Creates a new person with profile and backdrop image uploads.

**Content-Type:** `multipart/form-data`

**Form Fields:**

| Field                     | Type   | Required | Description                                          |
| ------------------------- | ------ | -------- | ---------------------------------------------------- |
| `name`                    | string | yes      | Person name (1-128 chars)                            |
| `roles`                   | string | yes      | Comma-separated roles: PERFORMER, CONTENT_CREATOR    |
| `stage_name`              | string | no       | Stage name (defaults to name if not provided)        |
| `bio`                     | string | no       | Biography (max 2000 chars)                           |
| `birth_date`              | string | no       | Birth date in YYYY-MM-DD (must be 18+ years old)     |
| `birth_place`             | string | no       | Birth place (max 256 chars)                          |
| `nationality`             | string | no       | Nationality (max 64 chars)                           |
| `gender`                  | string | yes      | Gender: Male, Female, Trans                          |
| `height`                  | int    | no       | Height in cm                                         |
| `active`                  | bool   | no       | Active status (defaults to true)                     |
| `debut_year`              | int    | no       | Debut year (1900-2100)                               |
| `career_status`           | string | yes      | Career status: Active, Retired, Hiatus               |
| `measurements_bust`       | int    | no       | Bust measurement                                     |
| `measurements_waist`      | int    | no       | Waist measurement                                    |
| `measurements_hips`       | int    | no       | Hips measurement                                     |
| `measurements_unit`       | string | no       | Unit: inches or cm                                   |
| `measurements_body_type`  | string | no       | Body type                                            |
| `measurements_eye_color`  | string | no       | Eye color                                            |
| `measurements_hair_color` | string | no       | Hair color                                           |
| `tags`                    | string | no       | Comma-separated tag names (auto-generates IDs)       |
| `categories`              | string | no       | Comma-separated category names (auto-generates IDs)  |
| `specialties`             | string | no       | Comma-separated specialty names (auto-generates IDs) |
| `profile`                 | file   | no       | Profile image (max 10MB)                             |
| `backdrop`                | file   | no       | Backdrop image (max 10MB)                            |

**Auto-Generated Fields:**

- `id`: 6-digit numeric ID
- `contentType`: Always set to PERSON
- `verified`: Always set to false
- `stats`: Initialized with zero values

**Example Request:**

```bash
curl -X POST http://localhost:8080/v1/persons \
  -F "name=Leonardo DiCaprio" \
  -F "roles=PERFORMER" \
  -F "bio=Academy Award-winning actor..." \
  -F "birth_date=1974-11-11" \
  -F "gender=Male" \
  -F "career_status=Active" \
  -F "profile=@profile.jpg" \
  -F "backdrop=@backdrop.jpg"
```

**Response (201):**

```json
{
  "status": 201,
  "message": "person created",
  "data": {
    "id": "231931",
    "contentType": "PERSON",
    "name": "Leonardo DiCaprio",
    "verified": false,
    "stats": {
      "totalProductions": 0,
      "totalViews": 0,
      "subscriberCount": 0,
      "followersCount": 0,
      "averageRating": 0
    }
  }
}
```

**Error Response (400 - Age validation):**

```json
{
  "status": 400,
  "message": "Key: 'Person.BirthDate' Error:Field validation for 'BirthDate' failed on the 'age18plus' tag"
}
```

### DELETE /v1/a/people/{peopleId}

Deletes a person by ID.

**URL Parameters:**

| Parameter  | Type   | Description          |
| ---------- | ------ | -------------------- |
| `peopleId` | string | The ID of the person |

**Response (200):**

```json
{
  "status": 200,
  "message": "person deleted"
}
```

### GET /v1/a/people/{peopleId}/content

Returns all content where the person is in the cast (admin view, no filtering).

**URL Parameters:**

| Parameter  | Type   | Description          |
| ---------- | ------ | -------------------- |
| `peopleId` | string | The ID of the person |

**Query Parameters:**

| Parameter | Type   | Required | Description                            |
| --------- | ------ | -------- | -------------------------------------- |
| `type`    | string | no       | Filter by contentType (movie, tv, all) |

### GET /v1/a/attributes

Returns all attributes (admin view).

### GET /v1/a/attributes/{attributeId}

Returns a single attribute by ID (admin view).

**URL Parameters:**

| Parameter     | Type   | Description             |
| ------------- | ------ | ----------------------- |
| `attributeId` | string | The ID of the attribute |

### POST /v1/a/attributes

Creates a new attribute.

### DELETE /v1/a/attributes/{attributeId}

Deletes an attribute by ID.

**URL Parameters:**

| Parameter     | Type   | Description             |
| ------------- | ------ | ----------------------- |
| `attributeId` | string | The ID of the attribute |

## Router-Level Errors

The API returns JSON for unmatched routes and unsupported methods.

- Unknown route: `404 Not Found`
- Unsupported method on a valid route: `405 Method Not Allowed`

## Validation Rules

### Movie Validation

#### Required Fields

- `overview`: 1-1000 characters
- `original_language`: Must be one of: en, hi, ja, ko, fr, es

#### Optional Fields with Validation

- `id`: 1-64 characters (auto-generated UUID if not provided)
- `title`: 1-128 characters
- `release_date`: YYYY-MM-DD format, between 2006-01-01 and today
- `first_air_date`: YYYY-MM-DD format, between 2006-01-01 and today
- `content_rating`: Must be 18_PLUS or 21_PLUS
- `content_type`: Must be MOVIE or TV
- `runtime`: Must be >= 0
- `tagline`: Maximum 255 characters
- `visibility`: Must be PUBLIC or PRIVATE (defaults to PUBLIC if not provided)
- `origin_country`: Each country code must be 2-8 characters
- `status`: Must be one of: RUMORED, PLANNED, IN_PRODUCTION, POST_PRODUCTION, RELEASED, ENDED, RETURNING_SERIES, CANCELED, PILOT

#### Cast Validation

- Only performer IDs are accepted (not names)
- Each cast ID must exist in the person table
- Names are automatically fetched from the person table
- If a performer ID doesn't exist, returns 400 error: `performer with id 'X' not found`

#### Entity References (Genres, Tags, Studios, MoodTags)

- Can provide just names: `Action,Drama,Thriller`
- Can provide ID:name pairs: `123456:Action,123457:Drama`
- If only name provided, 6-digit numeric ID is auto-generated
- ID: 1-64 characters
- Name: 1-128 characters (optional for casts, required for others)

#### File Uploads

- `poster`: Optional, max 10MB, allowed types: jpeg, png, webp, gif, avif
- `cover`: Optional, max 10MB, allowed types: jpeg, png, webp, gif, avif
- Uploaded to S3 at: `movies/{movieId}/poster.{ext}` and `movies/{movieId}/cover.{ext}`

#### Auto-Generated Fields

- `id`: UUID v4 format (e.g., `deb1f6e5-cc3d-4e55-80f6-3043a1b98692`)
- `visibility`: Defaults to PUBLIC if not provided
- `stats`: Initialized with `{ totalViews: 0, totalLikes: 0, averageRating: 0 }`
- `audit`: Initialized with `{ createdAt: <timestamp>, version: 1, isDeleted: false }`
- Entity IDs for genres, tags, studios, moodTags: 6-digit numeric (100000-999999)

### Person Validation

#### Required Fields

- `name`: 1-128 characters
- `roles`: At least one role required, must be PERFORMER or CONTENT_CREATOR (comma-separated)
- `gender`: Must be one of: Male, Female, Trans
- `career_status`: Must be one of: Active, Retired, Hiatus

#### Optional Fields with Validation

- `stage_name`: 1-128 characters (defaults to `name` if not provided)
- `bio`: Maximum 2000 characters
- `birth_date`: YYYY-MM-DD format, must be 18+ years old
- `birth_place`: Maximum 256 characters
- `nationality`: Maximum 64 characters
- `height`: Must be >= 0
- `debut_year`: Must be between 1900 and 2100
- `measurements_unit`: Must be inches or cm

#### Age Validation

- Birth date must result in age >= 18 years
- Calculated based on current date
- Accounts for birthday not yet occurred in current year
- Returns 400 error if under 18: `Key: 'Person.BirthDate' Error:Field validation for 'BirthDate' failed on the 'age18plus' tag`

#### Entity References (Tags, Categories, Specialties)

- Can provide just names: `Award Winning,Action Star`
- Can provide ID:name pairs: `700001:Award Winning,700002:Action Star`
- If only name provided, 6-digit numeric ID is auto-generated
- ID: 1-64 characters
- Name: 1-128 characters (optional)

#### File Uploads

- `profile`: Optional, max 10MB, allowed types: jpeg, png, webp, gif, avif
- `backdrop`: Optional, max 10MB, allowed types: jpeg, png, webp, gif, avif
- Uploaded to S3 at: `person/{personId}/profile.{ext}` and `person/{personId}/cover.{ext}`

#### Auto-Generated Fields

- `id`: 6-digit numeric (100000-999999)
- `contentType`: Always set to PERSON
- `verified`: Always set to false
- `stageName`: Defaults to `name` if not provided
- `stats`: Initialized with `{ totalProductions: 0, totalViews: 0, subscriberCount: 0, followersCount: 0, averageRating: 0 }`
- `audit`: Initialized with `{ createdAt: <timestamp>, version: 1, isDeleted: false }`
- Entity IDs for tags, categories, specialties: 6-digit numeric (100000-999999)

### Date Format Validation

All dates must be in `YYYY-MM-DD` format:

- Valid: `2010-07-16`, `1974-11-11`
- Invalid: `16-07-2010`, `07/16/2010`, `2010/07/16`

#### Movie Date Validation (daterange)

- `release_date` and `first_air_date` must be between 2006-01-01 and today
- Future dates are rejected
- Dates before 2006 are rejected

#### Person Date Validation (age18plus)

- `birth_date` must result in age >= 18 years
- Calculated from current date
- Future dates are rejected

### Common Validation Errors

#### Public Endpoint Filtering

All public GET endpoints (GET all movies, GET single movie, GET movies by person, GET movies by attribute, GET banner) automatically filter content to only return:

- Movies with `visibility = PUBLIC`
- Movies with `status = RELEASED` or `status = IN_PRODUCTION`

This ensures that only publicly available and released/in-production content is visible to end users.

#### 400 Bad Request - Validation Failed

```json
{
  "status": 400,
  "message": "Key: 'Movie.ReleaseDate' Error:Field validation for 'ReleaseDate' failed on the 'daterange' tag"
}
```

#### 400 Bad Request - Performer Not Found

```json
{
  "status": 400,
  "message": "performer with id '999999' not found"
}
```

#### 400 Bad Request - Age Validation

```json
{
  "status": 400,
  "message": "Key: 'Person.BirthDate' Error:Field validation for 'BirthDate' failed on the 'age18plus' tag"
}
```

#### 400 Bad Request - Invalid Date Format

```json
{
  "status": 400,
  "message": "release_date must be in YYYY-MM-DD format"
}
```

#### 400 Bad Request - File Too Large

```json
{
  "status": 400,
  "message": "poster file exceeds max size limit"
}
```

#### 409 Conflict - Duplicate ID

```json
{
  "status": 409,
  "message": "The resource already exists"
}
```

### ID Generation

#### Movie IDs

- Format: UUID v4
- Example: `deb1f6e5-cc3d-4e55-80f6-3043a1b98692`
- Auto-generated if not provided

#### Person IDs

- Format: 6-digit numeric
- Range: 100000-999999
- Example: `231931`
- Always auto-generated
- Formula: `((timestamp % 100000) * 10 + counter % 10) % 900000 + 100000`
- Thread-safe using atomic counter

#### Entity IDs (Genres, Tags, Studios, Categories, Specialties, MoodTags)

- Format: 6-digit numeric
- Range: 100000-999999
- Example: `624871`
- Auto-generated when only name is provided
- Uses same formula as Person IDs
