项目需要的文档在docs目录下

不会写东西的时候优先看docs -> examples -> codegraph读源码 -> grep/read读源码

## Agent / 框架相关文档

需要快速理解或回顾 **trpc-agent-go** 框架时，先读：

- `Agent专用/trpc-agent-go/README.md`（主索引，30 秒上手 + 速查表）
- `Agent专用/trpc-agent-go/09-EDITH实战指南.md`（项目实际怎么用框架）

需要深入某个模块时，去 `docs/trpc-agent-go/docs/mkdocs/zh/` 找对应官方文档（详见 `Agent专用/trpc-agent-go/README.md#6-文档索引`）。

## 前后端契约

> **铁律：后端新增/修改接口，必须先在 [types/api.ts](Agent专用/web/types/api.ts) 定义类型，再写 Go 端 struct，两边 JSON tag 严格对齐。**

- `types/api.ts` 是前后端唯一的类型真理来源，不要在后端 struct 里加字段但前端不声明
- Go struct `json:"xxx"` tag ↔ TS interface field name 必须一致（snake_case）
- 流式事件 (SSE) 的 6 种 event type 保持 `text | reasoning | tool_call | tool_result | error | done`，加新类型需要前后端同步改
