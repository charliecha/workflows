# PRD: Go URL 短地址服务

## Introduction

一个轻量级的 URL 缩短服务，用 Go 实现。用户提交长 URL，系统返回短码；访问短码时自动跳转到原始 URL，并记录访问统计数据（点击数、来源、时间）。所有数据存储在内存中，适合演示和开发环境使用。

## Goals

- 将任意长 URL 转换为 6 位短码
- 通过短码实现 HTTP 302 跳转到原始 URL
- 记录每次访问的点击数、Referer、时间戳
- 提供查询接口获取指定短码的统计数据
- 服务启动即可用，无需外部依赖

## User Stories

### US-001: 生成短链接
**Description:** As a 开发者, I want to 提交长 URL 获取短码, so that 我可以分享更简短的链接.

**Acceptance Criteria:**
- [ ] `POST /shorten` 接受 JSON body `{"url": "https://..."}`
- [ ] 返回 JSON `{"short_code": "xxxxxx", "short_url": "http://localhost:8080/xxxxxx"}`
- [ ] 对相同长 URL 多次请求返回相同短码（幂等）
- [ ] 输入空字符串或非 http/https URL 时返回 `400 Bad Request` 及错误信息
- [ ] `go build ./...` 通过

### US-002: 短码跳转
**Description:** As a 用户, I want to 访问短链接自动跳转, so that 我能到达原始页面.

**Acceptance Criteria:**
- [ ] `GET /:code` 返回 `302 Found`，`Location` header 指向原始 URL
- [ ] 访问不存在的短码返回 `404 Not Found` 及 JSON 错误信息
- [ ] `go build ./...` 通过

### US-003: 访问统计记录
**Description:** As a 开发者, I want to 每次跳转时自动记录访问信息, so that 我能追踪链接使用情况.

**Acceptance Criteria:**
- [ ] 每次成功跳转记录：时间戳、`Referer` header（可为空）、`User-Agent` header
- [ ] 点击计数原子递增（并发安全）
- [ ] `go build ./...` 通过

### US-004: 查询统计数据
**Description:** As a 开发者, I want to 查询某个短码的统计信息, so that 我能看到链接的访问情况.

**Acceptance Criteria:**
- [ ] `GET /:code/stats` 返回 JSON，包含 `short_code`、`original_url`、`total_clicks`、`created_at`、`accesses`（最近 100 条记录，含 `time`、`referer`、`user_agent`）
- [ ] 查询不存在的短码返回 `404 Not Found`
- [ ] `go build ./...` 通过

## Functional Requirements

- FR-1: 系统必须对输入 URL 做有效性校验，仅接受 `http://` 或 `https://` 开头的 URL
- FR-2: 系统必须对长 URL 做 MD5 哈希，取前 6 位 Base62 字符作为短码
- FR-3: 系统必须在内存中存储短码到原始 URL 的映射，进程重启后数据清空
- FR-4: 系统必须在每次跳转时以原子操作递增点击计数
- FR-5: 系统必须保留每个短码最近 100 条访问记录
- FR-6: 系统必须在发生哈希冲突时（相同短码对应不同 URL）追加递增后缀重试

## Non-Goals

- 不支持用户注册/登录
- 不支持自定义短码
- 不支持短链过期时间
- 不支持持久化存储（重启丢失数据）
- 不提供管理后台 UI
- 不支持批量生成

## Technical Considerations

- 使用标准库 `net/http` 或轻量路由库（如 `chi`）
- 并发安全：使用 `sync.RWMutex` 保护内存 map，`atomic` 保护计数器
- 短码生成：`md5(url)` → hex → 取前 6 位转 Base62
- 访问记录用环形缓冲或固定大小 slice（cap 100）

## Success Metrics

- `POST /shorten` 响应时间 < 10ms（本地）
- 并发 100 请求无 race condition（`go test -race` 通过）
- 所有接口有对应单元测试，覆盖正常路径和错误路径

## Open Questions

- 短码长度是否需要可配置？（当前固定 6 位）
- 是否需要 `DELETE /:code` 接口删除短链？
