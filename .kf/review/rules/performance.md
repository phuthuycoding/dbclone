# Performance Review Rules

## Queries & Data Access
- No N+1 queries (check for loops with DB calls inside)
- Use pagination for list endpoints
- Add indexes for frequently queried fields
- Prefer batch operations over individual operations

## Caching
- Identify cacheable data (read-heavy, slow-compute)
- Set appropriate TTL
- Invalidate cache on data mutation

## Network & I/O
- Prefer async/parallel for independent I/O
- Set timeouts on external calls
- Handle connection pooling properly
- Avoid unnecessary data transfer

## Memory
- Stream large data instead of loading into memory
- Watch for memory leaks in event listeners
- Clean up resources (close files, connections)

## Frontend (if applicable)
- Lazy load non-critical resources
- Optimize bundle size
- Minimize re-renders (React memoization)
- Compress images and assets
