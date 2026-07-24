# EDITH 项目说明

## 目录职责

- `backend/`：Go 后端，负责 Gateway、Runner、Sandbox、Telegram 和 HTTP 接口。
- `web/`：Next.js 前端与 BFF，负责 Clerk 登录、页面和 API 转发。
- `docs/`：EDITH 自己的架构设计与学习文档。
- `.claude/`、`.codex/`：项目级 Agent / Skill 配置，需要随仓库提交，不要删除或忽略。

## 阅读源码的顺序

不会写或需要理解现有能力时，按这个顺序查：

1. `docs/`：先看 EDITH 自己的设计和已有心智模型。
2. 相关框架的官方示例与文档。
3. CodeGraph：已准备参考源码时，优先用它理解调用关系。
4. `rg` / `read`：最后定位具体实现。

## Agent / 框架学习文档

需要快速回顾 tRPC-Agent-Go 时，按顺序阅读：

- `docs/learn/快速理解框架概念.md`
- `docs/learn/trpc-agent-go/01-核心心智模型.md`
- `docs/learn/trpc-agent-go/02-Runner.md` 至 `08-沙箱与Skill.md`

## 前后端契约

> **铁律：后端新增或修改接口，必须先在 `web/types/api.ts` 定义类型，再写 Go 端 struct；两边 JSON tag 严格对齐。**

- `web/types/api.ts` 是前后端交互类型的唯一真理来源。
- Go struct `json:"xxx"` tag ↔ TypeScript 字段名必须一致，使用 `snake_case`。
- 流式事件（SSE）保持 `text | reasoning | tool_call | tool_result | error | done`；新增类型时前后端必须同步修改。
