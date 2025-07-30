# Rate Limit Optimization Strategy

Based on the rate-limiting feedback, we need to dramatically reduce our API usage to work within the 10 requests/day limit for non-federal accounts.

## Key Constraints

1. **10 requests/day** for non-federal accounts (calendar day, UTC)
2. **1,000 requests/hour** rolling window across all api.data.gov services
3. All requests with the same key count toward the same quota
4. Pagination multiplies call count - each page is a separate request

## Optimization Strategy

### 1. Maximize Results Per Request

**Current**: limit=100 (default)
**Optimized**: limit=1000 (maximum allowed)

This gives us 10x more results per API call.

### 2. Use Delta Queries Only

Instead of searching all opportunities from the last N days, use:
- `updatedFrom/updatedTo` to get only recently modified opportunities
- Store the last successful query timestamp
- Query only changes since last run

### 3. Single Combined Query

With only 10 requests/day, we can't afford multiple search queries. Options:
- Combine all search terms into one broad query
- Use the most important search term only
- Rotate through different queries each day

### 4. Schedule Around UTC Boundaries

- **Morning run**: After 01:00 UTC (8 PM ET previous day)
- **Evening run**: After 13:00 UTC (8 AM ET same day)

This ensures each run uses a different day's quota.

### 5. Request Monitoring

Track and display:
- `X-RateLimit-Remaining` header
- `Retry-After` header on 429s
- Daily request counter in state file

## Implementation Plan

### Phase 1: Immediate Changes
1. Increase limit to 1000
2. Implement updatedFrom/To parameters
3. Combine queries into single request
4. Add UTC-aware scheduling

### Phase 2: Enhanced Tracking
1. Store last query timestamp
2. Track daily request count
3. Monitor rate limit headers
4. Add pre-flight check for remaining quota

### Phase 3: Long-term Solution
1. Request federal/system account status
2. Implement local caching database
3. Build incremental sync system

## Example Optimized Query

Instead of:
```yaml
queries:
  - name: "AI/ML"
    params:
      q: "artificial intelligence"
      limit: 100
  - name: "Cybersecurity" 
    params:
      q: "cybersecurity"
      limit: 100
```

Use:
```yaml
queries:
  - name: "Combined Tech"
    params:
      q: "artificial intelligence OR machine learning OR cybersecurity OR blockchain"
      limit: 1000
      updatedFrom: "{{ .LastQueryTime }}"
      updatedTo: "{{ .Now }}"
```

## Expected Results

With these optimizations:
- **Before**: 2-10 requests per run (200-1000 opportunities)
- **After**: 1 request per run (up to 1000 opportunities)
- **Daily total**: 2 requests (morning + evening)
- **Buffer**: 8 requests for retries/errors

This leaves significant headroom within the 10 request daily limit.