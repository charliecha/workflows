# 项目初始化：go mod、目录结构、chi 依赖

## Description

创建 Go URL 短地址服务的项目骨架。初始化 Go module，安装 chi 路由库，建立 `store/` 和 `handler/` 分包目录，确保项目可构建。

## Acceptance Criteria

- [ ] `go mod init` 完成，module 名为 `github.com/user/url-shortener`
- [ ] `go get github.com/go-chi/chi/v5` 安装成功，写入 `go.mod` / `go.sum`
- [ ] 目录结构符合 SPEC 2.3：`store/store.go`、`handler/handler.go`、`main.go` 均存在（可为空骨架）
- [ ] `go build ./...` 通过

## Dependencies

None

## Type

infra

## Priority

high

## SPEC Reference

Section 2.3 — File Structure
