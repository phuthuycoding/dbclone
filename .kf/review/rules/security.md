# Security Review Rules

## Secrets Management
- No hardcoded credentials, API keys, or tokens
- No secrets in workflow artifacts (specs, plans, reports) — `kf validate` rejects secret-like content
- Use environment variables or secret managers
- Never log sensitive data

## Threat Model First
- Before applying a security finding, state what the code stores, protects, and exposes
- Fix the real failure mode, not the abstract pattern — a finding without a reachable attack path is LOW
- Do not add auth/crypto/validation where the data is not sensitive and not externally reachable

## Input Validation
- Validate all external input at system boundary
- Sanitize before using in queries, commands, templates
- Use parameterized queries (no string concatenation for SQL)

## Authentication & Authorization
- Check auth before business logic
- Use established auth patterns (not custom crypto)
- Enforce least-privilege access

## Dependencies
- No known vulnerable dependencies
- Review new dependencies before adding
- Lock dependency versions

## Data Protection
- Encrypt sensitive data at rest and in transit
- Mask sensitive data in logs
- Apply rate limiting on public endpoints
