# go-cache-kit

[![CI](https://github.com/philiprehberger/go-cache-kit/actions/workflows/ci.yml/badge.svg)](https://github.com/philiprehberger/go-cache-kit/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/philiprehberger/go-cache-kit.svg)](https://pkg.go.dev/github.com/philiprehberger/go-cache-kit)
[![License](https://img.shields.io/github/license/philiprehberger/go-cache-kit)](LICENSE)

Generic in-memory LRU cache with TTL, tags, eviction callbacks, stats, and thread safety for Go

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

### Compute-on-Miss

```go
val := cache.GetOrSet("user:42", func() string {
    return fetchUserFromDB(42)
}, cachekit.WithTTL(10*time.Minute))
```

### Batch Retrieval

```go
results := cache.GetMany([]string{"user:1", "user:2", "user:3"})
// returns map[string]V with only the keys that exist and are not expired
```

### Eviction Callback

```go
cache.OnEvict(func(key string, value string) {
    log.Printf("evicted %s", key)
})
```

### Cache Stats

```go
stats := cache.Stats()
fmt.Printf("hits=%d misses=%d evictions=%d\n", stats.Hits, stats.Misses, stats.Evictions)
```

### Conditional Deletion

```go
removed := cache.DeleteWhere(func(key string, value string) bool {
    return strings.HasPrefix(key, "temp:")
})
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

## API

| Function / Method | Description |
|---|---|
| `New[V any](maxSize int, defaultTTL time.Duration) *Cache[V]` | Create a new LRU cache with max size and default TTL |
| `(*Cache[V]).Set(key string, value V, opts ...SetOption)` | Add or update an entry in the cache |
| `(*Cache[V]).Get(key string) (V, bool)` | Retrieve a value by key; returns false if missing or expired |
| `(*Cache[V]).Has(key string) bool` | Check if a key exists and is not expired |
| `(*Cache[V]).Delete(key string) bool` | Remove an entry by key; returns true if found |
| `(*Cache[V]).GetOrSet(key string, factory func() V, opts ...SetOption) V` | Return cached value or compute and cache it on miss |
| `(*Cache[V]).GetMany(keys []string) map[string]V` | Retrieve multiple values; omits missing or expired keys |
| `(*Cache[V]).InvalidateByTag(tag string) int` | Remove all entries with the given tag; returns count removed |
| `(*Cache[V]).DeleteWhere(predicate func(key string, value V) bool) int` | Remove all entries matching the predicate; returns count removed |
| `(*Cache[V]).Clear()` | Remove all entries from the cache |
| `(*Cache[V]).Size() int` | Return the current number of entries |
| `(*Cache[V]).Keys() []string` | Return all non-expired keys |
| `(*Cache[V]).OnEvict(fn func(key string, value V))` | Register a callback for eviction events |
| `(*Cache[V]).Stats() CacheStats` | Return hit, miss, and eviction counters |
| `WithTTL(d time.Duration) SetOption` | Override the default TTL for a Set call |
| `WithTags(tags ...string) SetOption` | Associate tags with an entry |
| `CacheStats` | Struct with `Hits`, `Misses`, and `Evictions` counters (int64) |
| `SetOption` | Functional option type for configuring Set calls |

## Development

```bash
go test ./...
go vet ./...
```

## License

MIT
