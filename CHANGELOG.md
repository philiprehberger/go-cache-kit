# Changelog

## 0.2.1

- Add tests for overwriting entries with different TTL and tags
- Add rapid sequential insertion stress test

## 0.2.0

- Fix `Keys()` to use read lock instead of write lock
- Add `maxSize` validation in `New()` (panics if <= 0)
- Add comprehensive test suite

## 0.1.0

- Initial release
