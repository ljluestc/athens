# PR Description — Fix uppercase module path handling for S3-compatible object stores

## Summary
This PR fixes module fetch failures for uppercase import paths when using S3-compatible backends that reject `!` in object keys (for example, Yandex Object Storage). Athens currently stores escaped uppercase module paths with `!` segments (Go module escaped path form), which is incompatible with those object-store key restrictions.

## Problem
Fetching modules with uppercase letters in their path can fail with 404/redirect loops in environments where object-storage keys cannot include `!`.

Example affected module:
- `github.com/ThomsonReutersEikon/go-ntlm`

Observed storage key pattern:
- `github.com/!thomson!reuters!eikon/go-ntlm/...`

Some S3-compatible providers disallow `!` in object keys, causing Athens storage/read operations to fail for these module paths.

## Root Cause
- Athens follows escaped module path conventions that encode uppercase letters with `!`.
- Object storage backends are assumed to accept those keys.
- Certain S3-compatible providers enforce stricter character policies and reject `!`, so persisted module artifacts become unreachable or never successfully written.

## Proposed Fix
Introduce storage-key normalization for escaped module paths when using affected object-storage drivers:

1. Detect escaped-path `!` segments in module-key construction.
2. Apply a reversible key-safe encoding strategy for restricted backends.
3. Use the same encoding/decoding consistently on write/read/list operations.
4. Keep Go module protocol responses unchanged so clients still see canonical module paths.

## Compatibility
- Existing modules and protocol behavior remain unchanged from client perspective.
- Changes are storage-layer focused and scoped to key construction/resolution logic.
- Backends that already accept `!` continue to work as before.

## Risk
- **Risk**: mismatch between encoded write keys and lookup keys can cause misses.
- **Mitigation**: centralize key transformation in one helper used by all S3 operations.
- **Risk**: migration behavior for pre-existing objects.
- **Mitigation**: support fallback lookup for legacy key style during transition, if needed.

## Validation Plan
- Unit tests for key normalization logic:
  - escaped uppercase path input
  - reversible round-trip transformation
  - no-op behavior for already-safe paths
- Integration tests against S3-compatible mock/minio-style storage:
  - upload/fetch/list for module paths containing uppercase letters
  - regression checks for lowercase-only module paths

## Manual Test
1. Configure Athens with S3-compatible backend that enforces strict key rules.
2. Request module with uppercase letters, e.g. `github.com/ThomsonReutersEikon/go-ntlm`.
3. Confirm successful download and persisted artifacts.
4. Confirm lowercase module requests remain unaffected.

## Rollout
- Merge with test coverage for key transformation behavior.
- Monitor storage misses and redirect/error logs after deployment.
- Document backend compatibility note for S3-compatible providers with stricter key policies.

## Checklist
- [x] Full local PR description drafted.
- [ ] Functional implementation committed.
- [ ] Unit/integration tests added and passing.
