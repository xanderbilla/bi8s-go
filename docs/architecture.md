# Architecture

This project follows a layered architecture with clear separation of concerns. Each layer has a single responsibility and communicates only with adjacent layers.

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
  DynamoDB/S3    (AWS)
```

### Handler Layer (internal/http/)

The handler is the entry point for HTTP requests. Its responsibilities:

- Parse request (URL params, form data, files)
- Delegate to service layer
- Format response (success or error)
- Use response package for consistent JSON envelopes
- Use errs package for error handling

Handlers do NOT:

- Contain business logic
- Talk directly to database
- Perform validation (delegates to validation package)

### Service Layer (internal/service/)

The service layer contains business logic and orchestrates operations. Its responsibilities:

- Validate business rules
- Coordinate between repositories
- Transform data
- Handle file uploads to S3
- Initialize default values (stats, audit, IDs)

For movies:

- Validates cast members exist in person table
- Populates cast names from person table
- Initializes stats (totalViews, totalLikes, averageRating)
- Generates UUID for movie ID

For persons:

- Generates 6-digit numeric ID
- Sets default values (verified=false, contentType=PERSON)
- Defaults stageName to name if not provided
- Initializes stats with zero values

### Repository Layer (internal/repository/)

The repository is the only layer that communicates with DynamoDB. Its responsibilities:

- Execute database operations (GetAll, Get, Create, Delete)
- Handle DynamoDB-specific logic (conditional writes, marshaling)
- Return domain models (not DynamoDB types)

Each repository is defined as an interface, making it easy to:

- Mock for testing
- Swap implementations
- Add caching layer

### Validation Package (internal/validation/)

Centralized validation logic:

- `validator.go` - Struct validation with custom validators
  - `daterange` - Validates dates between 2006-01-01 and today
  - `age18plus` - Validates person is 18+ years old
  - `customdate` - Custom date type validation

- `file.go` - File extraction and validation from multipart forms
  - Size limits
  - Content type validation
  - Empty file detection

- `performer.go` - Cast/performer validation
  - Validates performer exists in person table
  - Populates cast names from database

### Response Package (internal/response/)

Standardized API response format:

```go
type Envelope struct {
    Status  int         `json:"status"`
    Message string      `json:"message,omitempty"`
    Data    interface{} `json:"data,omitempty"`
    Error   string      `json:"error,omitempty"`
}
```

All responses use this envelope for consistency.

### Errs Package (internal/errs/)

Centralized error handling:

- Error definitions (ErrFileUploaderNotConfigured, ErrPerformerNotFound, etc.)
- HTTP error handlers (InternalServerError, BadRequestError, NotFoundError, ConflictError)
- Logs errors with request context
- Returns safe error messages to clients

## Application Container

The `app.Application` struct is created once at startup and holds:

- Configuration (environment variables)
- AWS clients (DynamoDB, S3)
- Services (MovieService, PersonService)

This avoids global variables and makes testing easier.

## Request Flow Examples

### POST /v1/movies (Create Movie)

1. Request arrives at router
2. Chi middleware runs (request ID, logger, recoverer, timeout, CORS)
3. `MovieHandler.CreateMovie` parses multipart form
4. Handler calls `ParseMovieFromForm` to build movie struct
5. Handler calls `validation.ValidateStruct` for field validation
6. Handler extracts poster and cover files
7. Handler calls `MovieService.Create`
8. Service validates casts using `validation.ValidateAndPopulateCasts`
   - Checks each cast ID exists in person table
   - Populates cast names from person table
   - Returns error if performer not found
9. Service generates movie ID (UUID)
10. Service initializes stats and audit fields
11. Service uploads poster to S3 at `movies/{id}/poster.{ext}`
12. Service uploads cover to S3 at `movies/{id}/cover.{ext}`
13. Service calls `MovieRepository.Create`
14. Repository marshals movie to DynamoDB format
15. Repository writes to DynamoDB with conditional check (no duplicate IDs)
16. Handler sends `201 Created` JSON response

### POST /v1/persons (Create Person)

1. Request arrives at router
2. Chi middleware runs
3. `PersonHandler.CreatePerson` parses multipart form
4. Handler calls `ParsePersonFromForm` to build person struct
5. Handler calls `validation.ValidateStruct`
   - Validates age is 18+ using `age18plus` validator
   - Validates birth date format
6. Handler extracts profile and backdrop files
7. Handler calls `PersonService.Create`
8. Service generates 6-digit numeric ID using `utils.GenerateNumericID()`
9. Service sets default values:
   - `contentType = PERSON`
   - `verified = false`
   - `stageName = name` (if not provided)
10. Service initializes stats with zero values
11. Service uploads profile to S3 at `person/{id}/profile.{ext}`
12. Service uploads backdrop to S3 at `person/{id}/cover.{ext}`
13. Service calls `PersonRepository.Create`
14. Repository writes to DynamoDB
15. Handler sends `201 Created` JSON response

### GET /v1/movies (List Movies)

1. Request arrives at router
2. Chi middleware runs
3. `MovieHandler.GetAllMovies` calls `MovieService.GetAll`
4. Service calls `MovieRepository.GetAll`
5. Repository scans DynamoDB table
6. Repository returns list of movies
7. Handler converts to `MoviePublicList` (filters out stats and audit)
8. Handler sends `200 OK` with public fields only:
   - id, title, backdropPath, posterPath, releaseDate
   - tags, contentRating, contentType, runtime

### GET /v1/movies/{id} (Get Single Movie)

1. Request arrives at router
2. Chi middleware runs
3. `MovieHandler.GetMovie` extracts ID from URL
4. Handler calls `MovieService.Get`
5. Service calls `MovieRepository.Get`
6. Repository queries DynamoDB by ID
7. Repository returns movie or nil
8. Handler converts to `MoviePublicDetail` (filters out stats and audit)
9. Handler sends `200 OK` with all fields except stats and audit

## ID Generation

### Movie IDs

- Generated using UUID v4
- Format: `deb1f6e5-cc3d-4e55-80f6-3043a1b98692`
- Function: `utils.GenerateID()`

### Person IDs

- Generated using 6-digit numeric formula
- Format: `231931` (100000-999999)
- Function: `utils.GenerateNumericID()`
- Formula: `((timestamp % 100000) * 10 + counter % 10) % 900000 + 100000`
- Thread-safe using atomic counter

### Entity IDs (Genres, Tags, Studios, etc.)

- Generated using same 6-digit numeric formula
- Auto-generated when only name is provided
- Format: `624871`

## Data Flow

```
Client Request
     ↓
Handler (parse, validate format)
     ↓
Validation Package (validate business rules)
     ↓
Service (business logic, orchestration)
     ↓
Validation Package (validate cross-entity rules)
     ↓
Repository (database operations)
     ↓
DynamoDB/S3
     ↓
Repository (unmarshal)
     ↓
Service (transform if needed)
     ↓
Handler (format response)
     ↓
Client Response
```

## Error Handling Flow

```
Error occurs in any layer
     ↓
Return error up the stack
     ↓
Handler catches error
     ↓
Handler checks error type:
  - Performer not found → 400 Bad Request
  - Validation error → 400 Bad Request
  - Duplicate ID → 409 Conflict
  - Not found → 404 Not Found
  - Other → 500 Internal Server Error
     ↓
Handler calls errs package function
     ↓
Errs package logs error with context
     ↓
Errs package calls response package
     ↓
Response package sends JSON envelope
     ↓
Client receives error response
```

## Testing Strategy

### Unit Tests

- Service layer: Mock repository interface
- Validation package: Test validators independently
- Utils: Test ID generation, date parsing

### Integration Tests

- Repository layer: Test against DynamoDB local
- Handler layer: Use httptest package

### End-to-End Tests

- Full request flow against test environment
- Verify S3 uploads
- Verify DynamoDB writes
