# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## 语言

始终使用中文回复。代码、命令、API 名称、报错信息等保持原文。

## Overview

`github.com/vicanso/go-cache/v2` (package `cache`) is a caching library that composes simple building blocks into multi-level caches with TTL and compression support. It is a flat, single-package library — every `.go` file at the repo root is part of `package cache`.

## Commands

```bash
make test        # go test -race -cover ./...
make test-cover  # writes test.out and opens the HTML coverage report
make lint        # golangci-lint run
make bench       # go test --benchmem -bench=. ./...
make hooks       # install hooks/pre-commit into .git/hooks (runs lint + test on commit)

# run a single test
go test -race -run TestRedisCache ./...
```

Tests are self-contained — `redis_test.go`'s `newClient(t)` starts an in-memory **miniredis** instance per test, so no external Redis is required. Caveat: miniredis does not expire keys on wall-clock time; tests that depend on TTL expiry advance it explicitly with `mr.FastForward(...)` (see `TestRedisLock`).

`go.mod` declares `go 1.24` and CI (`.github/workflows/test.yml`) builds against 1.24 only.

## Architecture

There are three independent public types; they share the `Store` abstraction and the marshal/timestamp helpers but are used separately.

### `Cache` (cache.go) — the multi-level cache

Constructed via `New(ttl, opts...)`. Configuration is functional-options only (`option.go`), never struct literals.

- **`stores []Store`** is an ordered chain: index 0 is the primary (fast/small, defaults to bigcache), and an optional secondary (`CacheSecondaryStoreOption`, typically redis) is appended at index 1. Reads walk the chain in order; writes fan out to *all* stores.
- **Back-fill on read**: `getBytes` reads from each store in turn. On a hit in a later store (index != 0) it writes the value back into the primary store, so a bigcache eviction or a cross-instance redis update re-populates the fast tier automatically.
- **Timestamp prefix is the core trick**: every value written by `Cache` is prefixed with an 8-byte big-endian `UnixNano` expiry (`time.go`, `timestampByteSize = 8`). This is how per-key TTL works *across* stores even though **bigcache only supports one instance-wide TTL** — bigcache's own expiry is effectively ignored and the embedded timestamp is the source of truth. `getBytes` recomputes remaining TTL from this and skips already-expired entries.
- **Per-level TTL**: `CacheMultiTTLOption([]time.Duration{...})` sets a different TTL per store index (e.g. short in memory, long in redis); `getTTL(index, ttl...)` resolves it, with an explicit call-site `ttl` argument overriding.
- **Generics**: package-level `Get[T](ctx, c, key)` returns `*T`; the method `Get` unmarshals into a caller-provided value.

### `RedisCache` (redis_cache.go) — direct redis helpers

A standalone convenience wrapper around `redis.UniversalClient` (not a `Store`). Forces a TTL on every write (`defaultRedisTTL` = 10m when unset) and prefixes keys. Adds redis-specific operations the generic `Cache` doesn't have: `Lock`/`LockWithDone` (SetNX-based), `IncWith` (SetNX+IncrBy in a TxPipeline), `GetAndDel`, and `SetStructWithTTL`/`GetStructAndTTL` (which use the same 8-byte timestamp-prefix convention as `Cache`).

### `RedisSession` (redis_session.go) — session store

Minimal `Get`/`Set`/`Destroy` over redis for use as an `elton` session backend. `Get` intentionally swallows `redis.Nil` and returns empty, not an error.

### Shared pieces

- **`Store` interface (store.go)**: `Set`/`Get`/`Delete`/`Close`. Implementations: `bigCacheStore` (big_cache_store.go, default; uses 8 shards by default since the common pattern is many small stores) and `redisStore` (redis_store.go). Both normalize a miss to `ErrIsNil`.
- **`Compressor` interface (compressor.go)**: snappy and zstd built-ins via `NewSnappyCompressor`/`NewZSTDCompressor` (or `CacheSnappyOption`/`CacheZSTDOption`). `Match(size)` gates compression on a minimum length; encoded output carries a 1-byte header (`CompressNone`/`Compressed`) so decode is self-describing. Compression sits *inside* `Cache` and wraps the value before the timestamp prefix is added.
- **marshal/unmarshal (marshaler.go, unmarshaler.go)**: JSON by default. A value may implement `Marshaler`/`Unmarshaler` to override, and `*bytes.Buffer` is special-cased to pass raw bytes through without JSON.

## Conventions

- Public methods resolve the prefixed key (`getKey`) once, then call unexported helpers (`getBytes`, `del`, `set`…) that assume the key is already prefixed — do not re-prefix inside the private helpers.
- All exported functions take `context.Context` as the first argument.
- Apache 2.0 license header on every source file.
