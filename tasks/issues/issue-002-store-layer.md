# store 层实现：Store、ShortLink、Storer 接口、generateCode、toBase62

## Description

实现内存存储层的全部核心逻辑：数据结构定义、短码生成算法（MD5 → Base62）、冲突处理、访问记录循环缓冲、并发安全保证。

## Acceptance Criteria

- [ ] `store.Store` 结构体实现 `Storer` 接口（`Shorten`、`Resolve`、`RecordAccess`、`Stats`）
- [ ] `Shorten` 对相同 URL 多次调用返回相同短码（幂等）
- [ ] `Shorten` 对哈希冲突最多重试 10 次（FR-6）
- [ ] `generateCode` 使用 MD5 + Base62，返回 6 位短码
- [ ] `toBase62` 使用 `0-9a-zA-Z` 字符集自实现
- [ ] `RecordAccess` 使用循环缓冲，最多保留 100 条记录，并发安全
- [ ] 点击计数使用 `atomic.Int64` 原子递增
- [ ] `go build ./...` 通过

## Dependencies

Issue #1

## Type

backend

## Priority

high

## SPEC Reference

Section 3.1 — Entity Definitions, Section 5.1~5.3 — Business Logic
