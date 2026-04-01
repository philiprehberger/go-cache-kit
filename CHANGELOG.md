# Changelog

## 0.3.2

- Standardize README to 3-badge format with emoji Support section
- Update CI checkout action to v5 for Node.js 24 compatibility
- Add GitHub issue templates, dependabot config, and PR template

## 0.3.1

- Consolidate README badges onto single line

## 0.3.0

- Add `GetOrSet` for compute-on-miss with optional `SetOption` support
- Add `GetMany` for batch retrieval of multiple keys
- Add `OnEvict` callback for LRU and TTL eviction notifications
- Add `Stats` method returning `CacheStats` with hit, miss, and eviction counters
- Add `DeleteWhere` for conditional deletion with a predicate function

## 0.2.2

- Add badges and Development section to README

## 0.2.1

- Add tests for overwriting entries with different TTL and tags
- Add rapid sequential insertion stress test

## 0.2.0

- Fix `Keys()` to use read lock instead of write lock
- Add `maxSize` validation in `New()` (panics if <= 0)
- Add comprehensive test suite

## 0.1.0

- Initial release
