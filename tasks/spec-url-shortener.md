# SPEC: Go URL 短地址服务

> 技术规范，来源于：`tasks/prd-url-shortener.md`
> 生成日期：2026-05-25 | 项目：greenfield

## 1. Summary

### 1.1 What This SPEC Covers

基于内存存储的 Go URL 短地址服务，提供短链生成、跳转、访问统计三个核心能力。使用 `chi` 路由库，分 `store` / `handler` 两包组织，无外部存储依赖。

### 1.2 PRD Reference

- Source: `tasks/prd-url-shortener.md`
- User Stories covered: US-001, US-002, US-003, US-004
- Functional Requirements covered: FR-1 ~ FR-6

### 1.3 Design Decisions Summary

| Decision | Choice | Rationale |
|----------|--------|-----------|
| 路由库 | `chi` | 兼容标准库 `http.Handler`，零学习成本，支持 URL 参数 |
| 项目结构 | `main` + `store` + `handler` 分包 | 可单独测试 store 层，handler 层通过接口注入 |
| 短码生成 | MD5 → hex → Base62 前 6 位 | 幂等，相同 URL 总得到相同短码 |
| Base62 字符集 | `0-9a-zA-Z` 标准 62 字符，自实现 | 无歧义，无需额外依赖 |
| 并发安全 | `sync.RWMutex` 保护 map，`sync/atomic` 保护计数 | 读多写少场景 RWMutex 性能更好 |
| 访问记录容量 | 固定 cap=100 的循环覆写 slice | 简单，无需额外数据结构 |

---

## 2. Architecture

### 2.1 System Context

```
Client
  │
  ├── POST /shorten        → handler.ShortenHandler → store.Store
  ├── GET  /:code          → handler.RedirectHandler → store.Store
  └── GET  /:code/stats    → handler.StatsHandler   → store.Store
```

### 2.2 Component Design

| 组件 | 职责 |
|------|------|
| `store.Store` | 内存存储，短码映射、访问记录、并发安全 |
| `handler` | HTTP 请求解析、响应序列化、错误处理 |
| `main` | 组装依赖，启动 HTTP server |

`handler` 层通过 `store.Storer` 接口访问 store，便于单元测试时 mock。

### 2.3 File Structure

```
url-shortener/
├── main.go                  [NEW] 启动入口，组装路由
├── go.mod                   [NEW] module: github.com/user/url-shortener
├── store/
│   ├── store.go             [NEW] Store 结构体 + Storer 接口
│   └── store_test.go        [NEW] store 层单元测试
└── handler/
    ├── handler.go           [NEW] HTTP handler 函数
    └── handler_test.go      [NEW] handler 层单元测试（httptest）
```

---

## 3. Data Model

### 3.1 Entity Definitions

```go
// store/store.go

type AccessRecord struct {
    Time      time.Time `json:"time"`
    Referer   string    `json:"referer"`
    UserAgent string    `json:"user_agent"`
}

type ShortLink struct {
    ShortCode   string         `json:"short_code"`
    OriginalURL string         `json:"original_url"`
    CreatedAt   time.Time      `json:"created_at"`
    clicks      atomic.Int64   // 不导出，通过方法访问
    accesses    [100]AccessRecord
    accessHead  int            // 循环写入位置
    accessCount int            // 实际记录数（上限 100）
    mu          sync.Mutex     // 保护 accesses/accessHead/accessCount
}

type Storer interface {
    Shorten(originalURL string) (shortCode string, err error)
    Resolve(shortCode string) (originalURL string, err error)
    RecordAccess(shortCode, referer, userAgent string) error
    Stats(shortCode string) (*StatsResponse, error)
}

type Store struct {
    mu    sync.RWMutex
    links map[string]*ShortLink // key: shortCode
    urls  map[string]string     // key: originalURL → shortCode（幂等查找）
}
```

### 3.2 响应结构

```go
// handler/handler.go

type ShortenRequest struct {
    URL string `json:"url"`
}

type ShortenResponse struct {
    ShortCode string `json:"short_code"`
    ShortURL  string `json:"short_url"`
}

type StatsResponse struct {
    ShortCode   string         `json:"short_code"`
    OriginalURL string         `json:"original_url"`
    TotalClicks int64          `json:"total_clicks"`
    CreatedAt   time.Time      `json:"created_at"`
    Accesses    []AccessRecord `json:"accesses"`
}

type ErrorResponse struct {
    Error string `json:"error"`
}
```

---

## 4. API Design

### 4.1 Endpoints

| Method | Path | Description | Request Body | Response |
|--------|------|-------------|-------------|---------|
| POST | `/shorten` | 生成短链 | `{"url":"..."}` | `200 ShortenResponse` |
| GET | `/:code` | 短码跳转 | — | `302 Location` |
| GET | `/:code/stats` | 查询统计 | — | `200 StatsResponse` |

### 4.2 Request/Response 示例

**POST /shorten**
```json
// Request
{"url": "https://www.example.com/very/long/path?query=value"}

// Response 200
{"short_code": "aB3xY9", "short_url": "http://localhost:8080/aB3xY9"}

// Response 400
{"error": "invalid URL: must start with http:// or https://"}
```

**GET /aB3xY9**
```
HTTP/1.1 302 Found
Location: https://www.example.com/very/long/path?query=value
```

**GET /aB3xY9/stats**
```json
{
  "short_code": "aB3xY9",
  "original_url": "https://www.example.com/very/long/path?query=value",
  "total_clicks": 42,
  "created_at": "2026-05-25T10:00:00Z",
  "accesses": [
    {"time": "2026-05-25T10:01:00Z", "referer": "https://google.com", "user_agent": "Mozilla/5.0..."}
  ]
}
```

### 4.3 Error Responses

| 场景 | HTTP Status | error 字段 |
|------|-------------|-----------|
| URL 为空 | 400 | `"url is required"` |
| URL 非 http/https | 400 | `"invalid URL: must start with http:// or https://"` |
| 短码不存在（跳转） | 404 | `"short code not found"` |
| 短码不存在（统计） | 404 | `"short code not found"` |
| 请求体解析失败 | 400 | `"invalid request body"` |

---

## 5. Business Logic

### 5.1 短码生成算法

```
func generateCode(originalURL string, attempt int) string:
    input = originalURL + ":" + strconv.Itoa(attempt)
    hash  = md5(input)
    hex   = hex.EncodeToString(hash)
    return toBase62(hex[:8])[:6]
```

**冲突处理（FR-6）：**
```
func (s *Store) Shorten(url string) (string, error):
    // 1. 先查 urls map，若已存在直接返回（幂等）
    if code, ok := s.urls[url]; ok { return code, nil }

    // 2. 尝试生成，最多 10 次
    for attempt := 0; attempt < 10; attempt++:
        code = generateCode(url, attempt)
        if _, ok := s.links[code]; !ok:
            s.links[code] = &ShortLink{...}
            s.urls[url] = code
            return code, nil
        else if existing.OriginalURL == url:
            return code, nil
        // 真冲突，attempt++ 继续

    return "", errors.New("failed to generate unique short code")
```

### 5.2 访问记录（循环写入）

```
func (l *ShortLink) recordAccess(referer, userAgent string):
    l.mu.Lock()
    defer l.mu.Unlock()
    l.accesses[l.accessHead] = AccessRecord{
        Time:      time.Now().UTC(),
        Referer:   referer,
        UserAgent: userAgent,
    }
    l.accessHead = (l.accessHead + 1) % 100
    if l.accessCount < 100 { l.accessCount++ }
```

读取时按时间顺序返回（最旧→最新），注意环绕处理。

### 5.3 Base62 转换

```
const base62Chars = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

func toBase62(hexStr string) string:
    // 将 hex 字符串解析为大整数，逐位对 62 取模，倒序拼接字符
```

### 5.4 Edge Cases

| 场景 | 处理方式 |
|------|---------|
| 相同 URL 并发 POST | RWMutex 写锁保护，第一个写入，后续命中 `urls` map |
| 短码 10 次冲突 | 返回 500 |
| accessCount 刚好 100 时读取 | 环形索引计算正确覆盖边界 |
| URL 带尾部斜杠 vs 不带 | 视为不同 URL（不做规范化） |

---

## 6. Error Handling

### 6.1 统一错误响应

```go
func writeJSON(w http.ResponseWriter, status int, v any)
func writeError(w http.ResponseWriter, status int, msg string)
```

### 6.2 Panic Recovery

`main.go` 中通过 `chi` 挂载 `middleware.Recoverer`。

---

## 7. Security

### 7.1 Input Validation

- URL 长度上限：2048 字节
- 仅接受 `http://` 或 `https://` scheme
- 使用 `net/url.Parse` 做结构校验

### 7.2 无认证

所有接口公开访问（PRD Non-Goals）。

---

## 8. Performance

### 8.1 Expected Load

演示/开发环境，目标 < 10ms P99 本地响应。

### 8.2 并发安全保证

- `Store.mu`（RWMutex）：`Shorten` 写锁，`Resolve`/`Stats` 读锁
- `ShortLink.clicks`（atomic.Int64）：无锁计数
- `ShortLink.mu`（Mutex）：保护 accesses 数组写入

---

## 9. Testing Strategy

### 9.1 Unit Tests — store 层

| 测试用例 | 验证点 |
|---------|--------|
| `TestShorten_NewURL` | 返回 6 位短码，写入两个 map |
| `TestShorten_Idempotent` | 相同 URL 两次调用返回相同短码 |
| `TestResolve_Found` | 返回正确原始 URL |
| `TestResolve_NotFound` | 返回 `ErrNotFound` |
| `TestRecordAccess_CircularBuffer` | 写入 101 条，验证最旧被覆盖 |
| `TestStats_ClickCount` | 并发 100 次 RecordAccess，TotalClicks == 100 |

### 9.2 Unit Tests — handler 层（httptest）

| 测试用例 | 验证点 |
|---------|--------|
| `TestShortenHandler_OK` | 200，返回 short_code 和 short_url |
| `TestShortenHandler_EmptyURL` | 400，error 字段正确 |
| `TestShortenHandler_InvalidURL` | 400，error 字段正确 |
| `TestRedirectHandler_Found` | 302，Location header 正确 |
| `TestRedirectHandler_NotFound` | 404，JSON error |
| `TestStatsHandler_OK` | 200，JSON 字段完整 |
| `TestStatsHandler_NotFound` | 404，JSON error |

### 9.3 Race Test

```bash
go test -race ./...
```

### 9.4 Acceptance Criteria Mapping

| US/FR | 测试 | 类型 |
|-------|------|------|
| US-001 | `TestShortenHandler_OK`, `TestShorten_Idempotent` | unit |
| US-001 | `TestShortenHandler_EmptyURL`, `TestShortenHandler_InvalidURL` | unit |
| US-002 | `TestRedirectHandler_Found`, `TestRedirectHandler_NotFound` | unit |
| US-003 | `TestStats_ClickCount`, `TestRecordAccess_CircularBuffer` | unit |
| US-004 | `TestStatsHandler_OK`, `TestStatsHandler_NotFound` | unit |
| FR-6 | `TestShorten_Conflict` | unit |

---

## 10. Implementation Plan

### 10.1 实现顺序

```
Phase 1: 项目脚手架（go mod init，安装 chi，目录结构）
Phase 2: store 层（Store、ShortLink、Storer 接口、generateCode、toBase62）
Phase 3: handler 层（ShortenHandler、RedirectHandler、StatsHandler）
Phase 4: main.go（组装路由，启动 :8080）
Phase 5: 验收（go build ./...，go test -race ./...，curl 手动验证）
```

### 10.2 Issue Mapping

| Issue | SPEC Sections | Depends On |
|-------|--------------|------------|
| #1 项目初始化 | 2.3 | — |
| #2 store 层实现 | 3, 5.1~5.3 | #1 |
| #3 handler 层实现 | 4, 6 | #2 |
| #4 main 组装 + 启动 | 2.1 | #3 |
| #5 测试补全 + race 验证 | 9 | #4 |

---

## 11. Open Questions & Risks

### 11.1 Unresolved Questions

- 短码长度是否可配置？（当前固定 6 位，可加常量 `codeLen = 6`）
- `DELETE /:code` 接口？（暂不实现）

### 11.2 Technical Risks

| Risk | Impact | Mitigation |
|------|--------|-----------|
| Base62 截取后碰撞率偏高 | 短码重复 | 6 位 Base62 = 56B+ 组合，最多重试 10 次 |
| 内存无界增长 | OOM（长时间运行） | 演示场景可接受 |

### 11.3 Assumptions

- 运行环境为单进程，无分布式部署需求
- `localhost:8080` 作为默认地址（可通过环境变量 `PORT` 覆盖）
- Go 版本 >= 1.21（使用 `atomic.Int64`）
