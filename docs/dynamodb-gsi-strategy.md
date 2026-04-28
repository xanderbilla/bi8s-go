# DynamoDB Global Secondary Index (GSI) Strategy

## Overview

This document defines the GSI strategy for production-grade DynamoDB performance. Without GSIs, the application uses expensive Scan operations that:

- Read entire tables (O(n) complexity)
- Charge per scanned item regardless of results
- Timeout at scale (>10k items)
- Cannot scale horizontally

## Current State (CRITICAL ISSUE)

**Problem:** All list operations use `Scan` with `FilterExpression`:

- `GetByContentId()` - Scans entire encoder table
- `GetAllMovies()` - Scans entire movies table
- `GetMoviesByGenre()` - Scans entire movies table
- `GetMoviesByYear()` - Scans entire movies table
- `GetAllPersons()` - Scans entire persons table
- `GetPersonsByRole()` - Scans entire persons table

**Impact:**

- 💰 **Cost**: Scans charge for ALL items read, not just returned
- ⏱️ **Performance**: Linear degradation as data grows
- 🚫 **Limits**: 1MB per Scan call requires pagination
- 📈 **Scale**: Will fail at 10k+ items

## Required GSIs

### 1. Encoder Table GSIs

#### GSI: `contentId-index`

**Purpose:** Query encoder jobs by content ID (movie/person)

```
Partition Key: contentId (String)
Sort Key: createdAt (Number) - timestamp for chronological ordering
Projection: ALL
```

**Replaces:** `GetByContentId()` Scan operation

**Query Pattern:**

```go
// Before (Scan - BAD)
Scan with FilterExpression: "contentId = :contentId"

// After (Query - GOOD)
Query on GSI with KeyConditionExpression: "contentId = :contentId"
```

**Cost Savings:** 100x reduction (only reads matching items)

---

### 2. Movies Table GSIs

#### GSI: `genre-releaseYear-index`

**Purpose:** Query movies by genre, optionally filtered by year

```
Partition Key: genre (String)
Sort Key: releaseYear (Number)
Projection: ALL
```

**Replaces:** `GetMoviesByGenre()` Scan operation

**Query Patterns:**

```go
// All movies in genre
Query with KeyConditionExpression: "genre = :genre"

// Movies in genre from specific year
Query with KeyConditionExpression: "genre = :genre AND releaseYear = :year"

// Movies in genre from year range
Query with KeyConditionExpression: "genre = :genre AND releaseYear BETWEEN :start AND :end"
```

#### GSI: `releaseYear-title-index`

**Purpose:** Query movies by year, sorted by title

```
Partition Key: releaseYear (Number)
Sort Key: title (String)
Projection: ALL
```

**Replaces:** `GetMoviesByYear()` Scan operation

**Query Pattern:**

```go
Query with KeyConditionExpression: "releaseYear = :year"
```

#### GSI: `status-updatedAt-index`

**Purpose:** Query movies by status (draft/published), sorted by update time

```
Partition Key: status (String)
Sort Key: updatedAt (Number)
Projection: ALL
```

**Use Cases:**

- Admin dashboard: "Show all draft movies"
- Content moderation: "Show recently updated movies"

---

### 3. Persons Table GSIs

#### GSI: `role-name-index`

**Purpose:** Query persons by role (actor/director/writer)

```
Partition Key: role (String)
Sort Key: name (String)
Projection: ALL
```

**Replaces:** `GetPersonsByRole()` Scan operation

**Query Pattern:**

```go
Query with KeyConditionExpression: "role = :role"
```

---

### 4. Attributes Table GSIs

#### GSI: `type-name-index`

**Purpose:** Query attributes by type (genre/language/country)

```
Partition Key: type (String)
Sort Key: name (String)
Projection: ALL
```

**Replaces:** `GetAttributesByType()` Scan operation

**Query Pattern:**

```go
Query with KeyConditionExpression: "type = :type"
```

---

## Implementation Priority

### Phase 1: Critical (Week 1)

1. ✅ Add `contentId-index` to encoder table
2. ✅ Add `genre-releaseYear-index` to movies table
3. ✅ Update repository methods to use Query instead of Scan
4. ✅ Add pagination support to API endpoints

### Phase 2: High Priority (Week 2)

1. Add `releaseYear-title-index` to movies table
2. Add `role-name-index` to persons table
3. Add `type-name-index` to attributes table
4. Update remaining repository methods

### Phase 3: Optimization (Week 3)

1. Add `status-updatedAt-index` for admin features
2. Monitor GSI usage and costs
3. Optimize projection types (ALL vs KEYS_ONLY vs INCLUDE)
4. Add caching layer if needed

---

## Terraform/OpenTofu Configuration

Add to `infra/tofu/modules/dynamodb/main.tf`:

```hcl
# Encoder Table GSI
resource "aws_dynamodb_table" "encoder" {
  # ... existing config ...

  global_secondary_index {
    name            = "contentId-index"
    hash_key        = "contentId"
    range_key       = "createdAt"
    projection_type = "ALL"
    read_capacity   = 5
    write_capacity  = 5
  }
}

# Movies Table GSIs
resource "aws_dynamodb_table" "movies" {
  # ... existing config ...

  global_secondary_index {
    name            = "genre-releaseYear-index"
    hash_key        = "genre"
    range_key       = "releaseYear"
    projection_type = "ALL"
    read_capacity   = 5
    write_capacity  = 5
  }

  global_secondary_index {
    name            = "releaseYear-title-index"
    hash_key        = "releaseYear"
    range_key       = "title"
    projection_type = "ALL"
    read_capacity   = 5
    write_capacity  = 5
  }

  global_secondary_index {
    name            = "status-updatedAt-index"
    hash_key        = "status"
    range_key       = "updatedAt"
    projection_type = "ALL"
    read_capacity   = 5
    write_capacity  = 5
  }
}

# Persons Table GSI
resource "aws_dynamodb_table" "persons" {
  # ... existing config ...

  global_secondary_index {
    name            = "role-name-index"
    hash_key        = "role"
    range_key       = "name"
    projection_type = "ALL"
    read_capacity   = 5
    write_capacity  = 5
  }
}

# Attributes Table GSI
resource "aws_dynamodb_table" "attributes" {
  # ... existing config ...

  global_secondary_index {
    name            = "type-name-index"
    hash_key        = "type"
    range_key       = "name"
    projection_type = "ALL"
    read_capacity   = 5
    write_capacity  = 5
  }
}
```

---

## Migration Strategy

### Step 1: Add GSIs (Zero Downtime)

```bash
cd infra/tofu/envs/prod
tofu plan
tofu apply
```

**Note:** GSI creation is online and non-blocking. Existing Scan operations continue working.

### Step 2: Deploy Code with Query Support

```bash
# Deploy new code that uses Query on GSIs
./scripts/deploy.sh
```

### Step 3: Monitor and Validate

```bash
# Check CloudWatch metrics
aws cloudwatch get-metric-statistics \
  --namespace AWS/DynamoDB \
  --metric-name ConsumedReadCapacityUnits \
  --dimensions Name=TableName,Value=movies Name=GlobalSecondaryIndexName,Value=genre-releaseYear-index \
  --start-time 2024-01-01T00:00:00Z \
  --end-time 2024-01-02T00:00:00Z \
  --period 3600 \
  --statistics Sum
```

### Step 4: Remove Scan Fallbacks (Optional)

Once GSIs are validated, remove Scan-based fallback code.

---

## Cost Analysis

### Before GSIs (Scan-based)

- **10,000 movies**, query by genre "Action" (1,000 matches)
- **Scan reads:** 10,000 items × 4KB = 40MB
- **Cost:** 40MB / 4KB = 10,000 RCUs
- **Time:** ~2-3 seconds with pagination

### After GSIs (Query-based)

- **Query reads:** 1,000 items × 4KB = 4MB
- **Cost:** 4MB / 4KB = 1,000 RCUs
- **Time:** ~100-200ms

**Savings:** 90% cost reduction, 10-20x faster

---

## Pagination Best Practices

### Query with Pagination

```go
func (r *DynamoMovieRepository) GetMoviesByGenre(ctx context.Context, genre string, limit int, lastKey map[string]types.AttributeValue) ([]model.Movie, map[string]types.AttributeValue, error) {
    input := &dynamodb.QueryInput{
        TableName:              aws.String(r.tableName),
        IndexName:              aws.String("genre-releaseYear-index"),
        KeyConditionExpression: aws.String("genre = :genre"),
        ExpressionAttributeValues: map[string]types.AttributeValue{
            ":genre": &types.AttributeValueMemberS{Value: genre},
        },
        Limit:             aws.Int32(int32(limit)),
        ExclusiveStartKey: lastKey,
    }

    result, err := r.client.Query(ctx, input)
    if err != nil {
        return nil, nil, err
    }

    var movies []model.Movie
    if err := attributevalue.UnmarshalListOfMaps(result.Items, &movies); err != nil {
        return nil, nil, err
    }

    return movies, result.LastEvaluatedKey, nil
}
```

### API Response with Pagination

```go
type PaginatedResponse struct {
    Items      []model.Movie `json:"items"`
    NextToken  *string       `json:"nextToken,omitempty"`
    TotalCount *int          `json:"totalCount,omitempty"` // Optional, expensive
}
```

---

## Monitoring and Alerts

### CloudWatch Alarms

1. **High Scan Count** - Alert if Scan operations > 100/min
2. **Throttled Requests** - Alert if throttling occurs
3. **High Latency** - Alert if p99 > 500ms
4. **GSI Capacity** - Alert if consumed > 80% provisioned

### Metrics to Track

- `ConsumedReadCapacityUnits` per table and GSI
- `UserErrors` (throttling)
- `SystemErrors`
- Query latency (p50, p99, p999)

---

## Testing Strategy

### Unit Tests

```go
func TestGetMoviesByGenre_UsesGSI(t *testing.T) {
    // Verify Query is called with correct IndexName
    // Verify KeyConditionExpression is correct
    // Verify pagination works
}
```

### Integration Tests

```go
func TestGetMoviesByGenre_Integration(t *testing.T) {
    // Insert test data
    // Query by genre
    // Verify results
    // Verify pagination
}
```

### Load Tests

```bash
# Simulate 1000 concurrent requests
ab -n 10000 -c 1000 https://api.example.com/v1/movies?genre=Action
```

---

## Rollback Plan

If GSIs cause issues:

1. **Immediate:** Revert code to use Scan (already deployed)
2. **Short-term:** Investigate GSI issues, fix, redeploy
3. **Long-term:** If GSIs are fundamentally broken, consider:
   - ElastiCache for query results
   - Read replicas with different access patterns
   - Migrate to Aurora/PostgreSQL

---

## References

- [DynamoDB GSI Best Practices](https://docs.aws.amazon.com/amazondynamodb/latest/developerguide/bp-indexes-general.html)
- [Query vs Scan Performance](https://docs.aws.amazon.com/amazondynamodb/latest/developerguide/bp-query-scan.html)
- [GSI Cost Optimization](https://aws.amazon.com/blogs/database/cost-optimization-for-amazon-dynamodb/)
