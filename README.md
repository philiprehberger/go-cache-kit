# go-cache-kit

[![CI](https://github.com/philiprehberger/go-cache-kit/actions/workflows/ci.yml/badge.svg)](https://github.com/philiprehberger/go-cache-kit/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/philiprehberger/go-cache-kit.svg)](https://pkg.go.dev/github.com/philiprehberger/go-cache-kit)
[![License](https://img.shields.io/github/license/philiprehberger/go-cache-kit)](LICENSE)

Generic in-memory LRU cache with TTL, tags, and thread safety for Go.

## Installation

```bash
go get github.com/philiprehberger/go-cache-kit
```

## Usage

### Basic Cache

```go
import "github.com/philiprehberger/go-cache-kit"

cache := cachekit.New[string](1000, 5*time.Minute)

cache.Set("key", "value")
val, ok := cache.Get("key") // "value", true
```

### Custom TTL per Entry

```go
cache.Set("session", "abc123", cachekit.WithTTL(30*time.Minute))
cache.Set("temp", "data", cachekit.WithTTL(5*time.Second))
```

### Tag-Based Invalidation

```go
cache.Set("user:1", userData, cachekit.WithTags("users", "team-a"))
cache.Set("user:2", userData, cachekit.WithTags("users", "team-b"))
cache.Set("post:1", postData, cachekit.WithTags("posts", "team-a"))

removed := cache.InvalidateByTag("team-a") // 2
```

### LRU Eviction

```go
cache := cachekit.New[string](100, 0) // no default TTL

// When full, least recently used entries are evicted
// Expired entries are preferred for eviction
```

### Other Operations

```go
cache.Has("key")      // check existence
cache.Delete("key")   // delete single entry
cache.Keys()          // list all non-expired keys
cache.Size()          // current entry count
cache.Clear()         // remove everything
```

## Development

```bash
go test ./...
go vet ./...
```

## License

MIT
