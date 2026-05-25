# handler 层实现：ShortenHandler、RedirectHandler、StatsHandler

## Description

实现三个 HTTP handler，通过 `store.Storer` 接口与 store 层交互。包含请求解析、URL 校验、统一错误响应格式。

## Acceptance Criteria

- [ ] `POST /shorten` 接受 `{"url":"..."}` 返回 `{"short_code":"...","short_url":"..."}`
- [ ] `POST /shorten` 对空 URL 返回 `400 {"error":"url is required"}`
- [ ] `POST /shorten` 对非 http/https URL 返回 `400 {"error":"invalid URL: must start with http:// or https://"}`
- [ ] `POST /shorten` 对非法请求体返回 `400 {"error":"invalid request body"}`
- [ ] URL 长度超过 2048 字节返回 `400`
- [ ] `GET /:code` 返回 `302 Found`，`Location` header 指向原始 URL
- [ ] `GET /:code` 对不存在短码返回 `404 {"error":"short code not found"}`
- [ ] `GET /:code/stats` 返回完整 `StatsResponse` JSON
- [ ] `GET /:code/stats` 对不存在短码返回 `404 {"error":"short code not found"}`
- [ ] `writeJSON` / `writeError` 辅助函数统一设置 `Content-Type: application/json`
- [ ] `go build ./...` 通过

## Dependencies

Issue #2

## Type

backend

## Priority

high

## SPEC Reference

Section 4 — API Design, Section 6 — Error Handling, Section 7.1 — Input Validation
