# feat: offline package synchronization tool for air-gapped Athens deployments

Closes #2117

## Problem

When Athens is deployed offline in enterprise/team environments (completely isolated from the internet), there is no built-in way to export packages from one Athens instance, transfer them via storage media, and import them into another. Users must manually copy, rename, and place files in the correct storage layout — which is error-prone and cumbersome.

## Solution

Add a standalone `athens-offline-sync` CLI tool with three subcommands:

- **`export`** — Reads modules from an Athens storage backend and writes them to a single `.tar.gz` archive with a `manifest.json` describing every included module@version.
- **`import`** — Reads an archive produced by `export` and saves all contained modules into the target Athens storage backend.
- **`delete`** — Removes specific module@version entries from a storage backend.

### Archive format (`athens-offline-sync/v1`)

```
manifest.json          # metadata: format_version, created_at, item_count, items[]
objects/000001/info.json
objects/000001/go.mod
objects/000001/source.zip
objects/000002/...
```

The archive is a gzip-compressed tar. The manifest maps each numbered directory back to its `module@version`, making the format self-describing and portable.

### Usage examples

```bash
# Export specific packages
athens-offline-sync export \
  -config_file /etc/athens/athens.toml \
  -package github.com/acme/lib@v1.2.3 \
  -package github.com/acme/util@v0.5.0 \
  -out packages.tar.gz

# Export entire catalog
athens-offline-sync export -config_file /etc/athens/athens.toml

# Import on air-gapped machine
athens-offline-sync import \
  -config_file /etc/athens/athens.toml \
  -in packages.tar.gz

# Delete deprecated versions
athens-offline-sync delete \
  -config_file /etc/athens/athens.toml \
  -package github.com/acme/old@v0.1.0
```

## Files added

**Core library:**
- `pkg/offlinesync/sync.go` — `ExportArchive()`, `ImportArchive()`, `DeletePackages()`, manifest types, tar/gzip I/O, catalog pagination, `module@version` parsing.

**CLI entrypoint:**
- `cmd/offline-sync/main.go` — `export`, `import`, `delete` subcommands with flag parsing, config loading, and storage backend initialization via existing `actions.GetStorage()`.

**Tests:**
- `pkg/offlinesync/sync_test.go` — 4 tests:
  - `TestParsePackageRef` — valid `module@version` parsing
  - `TestParsePackageRefInvalid` — rejects missing `@version`
  - `TestExportImportArchiveRoundTrip` — seeds in-memory storage, exports to archive, imports into fresh storage, verifies `.info` and `go.mod` match
  - `TestDeletePackages` — saves a module, deletes it, confirms count

## Type of Change

- [ ] Bug fix (non-breaking change which fixes an issue)
- [x] New feature (non-breaking change which adds functionality)
- [ ] Breaking change (fix or feature that would cause existing functionality to not work as expected)

## How Has This Been Tested?

- [x] Added new unit/integration tests
- [x] `go build ./pkg/offlinesync/` — passed
- [x] `go build ./cmd/offline-sync/` — passed
- [x] `go vet ./pkg/offlinesync/` — passed
- [x] `go test ./pkg/offlinesync/ -v` — 4/4 PASS

## Design decisions

- **Separate binary** rather than adding subcommands to the proxy. The proxy is a server; offline sync is an admin CLI tool. This avoids bloating the proxy binary.
- **Reuses `storage.Backend` interface** — works with any configured backend (disk, S3, GCS, Azure Blob, Minio, Mongo) without new storage code.
- **No new dependencies** — only uses stdlib `archive/tar`, `compress/gzip`, `encoding/json`, and existing Athens packages.
- **Full catalog export** is supported when `--package` flags are omitted, using the `Cataloger` interface. Falls back with a clear error if the backend doesn't implement it.

## Risk / Impact

**Low.** This is a new, additive binary. No existing code paths are modified. The proxy server is unchanged.

## Checklist

- [x] My code follows the style guidelines of this project
- [x] I have performed a self-review of my code
- [x] I have commented my code, particularly in hard-to-understand areas
- [x] My changes generate no new warnings
- [x] I have added tests that prove my feature works
