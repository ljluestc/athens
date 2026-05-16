---
title: "Offline package synchronization"
---

Athens now includes an offline synchronization tool for package export/import and cleanup workflows in isolated environments.

## Command

```bash
go run ./cmd/offline-sync --help
```

## Export

Export specific package versions:

```bash
go run ./cmd/offline-sync export \
  --config_file athens.toml \
  --out athens-offline-sync.tar.gz \
  --package github.com/acme/lib@v1.2.3 \
  --package github.com/acme/other@v0.9.0
```

Export the full catalog (for backends that support catalog listing):

```bash
go run ./cmd/offline-sync export \
  --config_file athens.toml \
  --out athens-offline-sync.tar.gz
```

## Import

```bash
go run ./cmd/offline-sync import \
  --config_file athens.toml \
  --in athens-offline-sync.tar.gz
```

## Delete package versions

```bash
go run ./cmd/offline-sync delete \
  --config_file athens.toml \
  --package github.com/acme/lib@v1.2.3 \
  --package github.com/acme/old@v0.1.0
```

By default, `delete` ignores missing package versions (`--ignore_missing=true`).
