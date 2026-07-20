# Huma SSE 前端集成指南

> 本文档记录 Huma + SSE 的前端集成方案，用于解决 Agent 应用前后端字段不一致的问题。

---

## 核心技术栈

| 层级 | 工具 | 作用 |
|------|------|------|
| 后端 | **Huma** (Go) | 基于 OpenAPI 的 REST/SSE 框架 |
| 文档 | **openapi.yaml** | 自动生成的 API 规范（机器可读） |
| 类型 | **openapi-typescript** | 从 openapi.yaml 生成 TS 类型定义 |
| HTTP | **openapi-fetch** | 用生成的类型调用 REST API |
| SSE | **@microsoft/fetch-event-source** | 处理 SSE 流式请求 |

---

## 工作流

```
1. 后端写 Huma 代码
       ↓
2. 自动生成 openapi.yaml
       ↓
3. 前端用 openapi-typescript 生成 api.ts（类型定义）
       ↓
4. 前端开发：
   - 普通 API → openapi-fetch + api.ts 类型
   - SSE → @microsoft/fetch-event-source + api.ts 类型
```

---

## 快速开始

### 1. 后端 SSE 示例 (Go + Huma)

```go
package main

import (
    "context"
    "net/http"
    "time"

    "github.com/danielgtaylor/huma/v2"
    "github.com/danielgtaylor/huma/v2/adapters/humago"
    "github.com/danielgtaylor/huma/v2/sse"
)

// 事件类型（前后端共用同一个结构）
type StreamEvent struct {
    Type    string `json:"type"`    // "text" | "done"
    Content string `json:"content"` // 文本内容
}

func main() {
    router := http.NewServeMux()
    api := humago.New(router, huma.DefaultConfig("Agent API", "1.0.0"))

    // SSE 端点
    sse.Register(api, huma.Operation{
        OperationID: "stream-chat",
        Method:      http.MethodGet,
        Path:        "/stream",
        Summary:     "聊天流",
    }, map[string]any{
        "message": StreamEvent{}, // 事件类型映射
    }, func(ctx context.Context, input *struct{}, send sse.Sender) {
        for _, ch := range "你好，世界！" {
            send.Data(StreamEvent{Type: "text", Content: string(ch)})
            time.Sleep(100 * time.Millisecond)
        }
        send.Data(StreamEvent{Type: "done", Content: ""}) // 结束标记
    })

    http.ListenAndServe(":8080", router)
}
```

### 2. 前端生成 SDK

```bash
# 安装工具
npm install openapi-typescript openapi-fetch @microsoft/fetch-event-source

# 生成类型定义
npx openapi-typescript http://localhost:8080/openapi.yaml -o ./src/api.ts
```

### 3. 前端使用

```typescript
import { fetchEventSource } from '@microsoft/fetch-event-source'
import { createClient } from 'openapi-fetch'
import type { paths, components } from './api'

// REST API 客户端
const client = createClient<paths>({ baseUrl: 'http://localhost:8080' })

// SSE 类型（从 api.ts 导入）
type StreamEvent = components['schemas']['StreamEvent']

// SSE 连接
const ctrl = new AbortController()
await fetchEventSource('http://localhost:8080/stream', {
    signal: ctrl.signal,
    onmessage(event) {
        const data: StreamEvent = JSON.parse(event.data)
        if (data.type === 'text') {
            appendToScreen(data.content) // 打字机效果
        } else if (data.type === 'done') {
            console.log('完成')
        }
    },
    onerror(err) {
        console.error('SSE 错误', err)
    },
})

// 取消请求
ctrl.abort()
```

---

## 关键概念

### Huma Register 的本质

```
huma.Register(api, operation, handler)
       │        │          │
       │        │          └── 你的业务函数（return = 响应）
       │        └── 路由元信息（Path, Method, Summary...）
       └── Huma API 实例（存储路由表，生成 OpenAPI）
```

### SSE Handler vs 普通 Handler

| | 普通 Handler | SSE Handler |
|--|-------------|-------------|
| 返回方式 | `return output` | `send.Data(event)` |
| 调用次数 | 一次 | 多次 |
| 连接 | 短连接 | 长连接 |

### SSE 事件类型映射

SSE 协议支持多种命名事件，`sse.Register` 的 `eventTypes` 参数声明了"我会发哪些事件"：

```go
sse.Register(api, op, map[string]any{
    "message":   MessageEvent{},   // 事件名 → Go 结构体
    "userJoin":  UserJoinEvent{},
}, handler)
```

前端收到时会标明事件名，可以监听特定事件。

---

## 类型安全的价值

```
后端改了字段名
    ↓
前端重新生成 api.ts
    ↓
TypeScript 编译报错
    ↓
前端自己知道要改
```

**不需要前后端沟通，类型就是文档，错误就是信号。**

---

## 路由适配器

Huma 支持多种底层路由：

| 适配器 | 底层路由 |
|--------|---------|
| `humago` | Go 标准库 `http.ServeMux` (Go 1.22+) |
| `humachi` | chi 路由库 |
| `humagin` | gin 框架 |

```go
// 用 Go 标准库（推荐 Go 1.22+）
router := http.NewServeMux()
api := humago.New(router, config)

// 而不是
router := chi.NewMux()
api := humachi.New(router, config)
```

---

## OpenAPI 文档

| 地址 | 说明 |
|------|------|
| `http://localhost:8080/openapi.yaml` | OpenAPI 规范文件（给工具读） |
| `http://localhost:8080/docs` | Swagger UI（给人看） |

**注意**：Swagger UI 不支持 SSE 测试，需要用 curl 或前端代码测试。

---

## 常见问题

**Q: Swagger UI 能测试 SSE 吗？**
A: 不能。SSE 是长连接，Swagger UI 的交互模型是"请求-响应"，不适用。

**Q: 普通 API 和 SSE 能用同一个 api.ts 吗？**
A: 能。openapi-typescript 只生成类型定义，不管你怎么发请求。

**Q: 后端改了字段，前端会知道吗？**
A: 会。重新生成 api.ts 后，TypeScript 编译时会报错。

---

## 相关资源

- [Huma 官方文档](https://huma.rocks)
- [openapi-typescript](https://www.npmjs.com/package/openapi-typescript)
- [@microsoft/fetch-event-source](https://www.npmjs.com/package/@microsoft/fetch-event-source)
