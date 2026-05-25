# 测试补全：store + handler 单元测试、go test -race 验证

## Description

补全 store 层和 handler 层的单元测试，覆盖正常路径和错误路径，并通过 race detector 验证并发安全。

## Acceptance Criteria

- [ ] `store/store_test.go`：`TestShorten_NewURL` — 返回 6 位短码
- [ ] `store/store_test.go`：`TestShorten_Idempotent` — 相同 URL 两次返回相同短码
- [ ] `store/store_test.go`：`TestResolve_Found` — 返回正确原始 URL
- [ ] `store/store_test.go`：`TestResolve_NotFound` — 返回错误
- [ ] `store/store_test.go`：`TestRecordAccess_CircularBuffer` — 写入 101 条，最旧被覆盖
- [ ] `store/store_test.go`：`TestStats_ClickCount` — 并发 100 次 RecordAccess，TotalClicks == 100
- [ ] `handler/handler_test.go`：使用 `httptest` 覆盖全部 7 个 handler 测试用例
- [ ] `go test -race ./...` 通过，无 data race 报告
- [ ] `go test ./...` 全部绿色

## Dependencies

Issue #4

## Type

backend

## Priority

medium

## SPEC Reference

Section 9 — Testing Strategy
