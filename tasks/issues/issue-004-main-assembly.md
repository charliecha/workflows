# main.go 组装路由并启动服务

## Description

在 `main.go` 中组装所有依赖：初始化 `store.Store`，注册 chi 路由，挂载 `middleware.Recoverer`，监听端口启动服务。

## Acceptance Criteria

- [ ] `main.go` 初始化 `store.NewStore()`，将其注入 handler
- [ ] chi 路由注册三个端点：`POST /shorten`、`GET /{code}`、`GET /{code}/stats`
- [ ] 挂载 `middleware.Recoverer` 防止 panic 崩溃服务
- [ ] 默认监听 `:8080`，支持通过环境变量 `PORT` 覆盖
- [ ] `go build ./...` 通过
- [ ] `go run . &` 启动后 `curl -X POST http://localhost:8080/shorten -d '{"url":"https://example.com"}'` 返回 200

## Dependencies

Issue #3

## Type

backend

## Priority

high

## SPEC Reference

Section 2.1 — System Context, Section 2.2 — Component Design
