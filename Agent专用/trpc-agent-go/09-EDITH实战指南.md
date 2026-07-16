# 09 - EDITH 实战指南

> EDITH = GithubAgent。本指南说明项目里 trpc-agent-go 怎么用、常见坑怎么避。

---

## 1. EDITH 是什么

GitHub Issue / PR 自动化 Agent。用户 `@EDITH` 触发 → 自动分析 → 提 PR → 通知。

**核心特征**：
- 单一租户 = 一个 Clerk User
- Web 实时对话 + GitHub Webhook 触发，共用同一 Runner
- E2B 远程沙箱执行代码
- AG-UI 协议推送到 Web 前端

---

## 2. 实际技术栈

```
go.mod 关键依赖：
  trpc.group/trpc-go/trpc-agent-go         v1.6.1-0.20260311094958-7b74ee59e339
  trpc.group/trpc-go/trpc-agent-go/server/agui v1.10.0
  trpc.group/trpc-go/trpc-agent-go/session/sqlite v1.10.0
  github.com/eric642/e2b-go-sdk            v0.1.3          ← 自封装 E2B SDK
  github.com/google/go-github/v69          v69.2.0
  github.com/golang-jwt/jwt/v5             v5.3.1
  github.com/mattn/go-sqlite3              v1.14.32
  connectrpc.com/connect                   v1.19.2
  github.com/ag-ui-protocol/ag-ui/sdks/community/go v0.0.0-...
```

**版本差异提醒**：核心库 `v1.6.1`，AG-UI 路径独立到 `v1.10.0`。两个版本在 go.mod 里同时存在是正常的（AG-UI Server 是独立 module），但要注意 API 兼容性。如果升级 core，注意 AG-UI 的 Resolver / Translator API 是否同步升级。

---

## 3. Runner 核心映射

```text
appName   = "edith"
userID    = Clerk user_id（Web 鉴权后取；GitHub 触发时映射为 installation_id 对应的用户）
sessionID = Web: RunAgentInput.threadId
            GitHub: "github:{owner}/{repo}#{number}"
requestID = EDITH run_id（UUID）
```

**同一 Issue / PR 用同一 Session**：连续 `@EDITH` 共享历史。

---

## 4. Web 与 GitHub 共用同一 AG-UI Runner

### 4.1 架构

```
Web 页面 ──SSE──┐
                ├─→ 共享 AG-UI Runner ─→ Core Runner ─→ Agent + Tools
GitHub Webhook ─┘                                            │
                                                               └→ GitHub API
```

**已经验证**（见 `forward/verify_agui/验证报告.md`）：两边能进入同一 `MessagesSnapshot`，由 `/history` 统一展示。

### 4.2 入口区分

| 入口 | 方式 | 限制 |
|---|---|---|
| Web | AG-UI 客户端 → SSE Service → AG-UI Runner | Run 期间禁止再次发消息（MVP 不加 /enqueue） |
| GitHub | Webhook → GitHub Handler → GitHubTrigger → Runner.EnqueueUserMessage | 后台静默跑完 |

### 4.3 共享实例

```go
// 服务启动时只创建一次
coreRunner := runner.NewRunner("edith", agent)

sharedAGUIRunner := aguirunner.New(
    coreRunner,
    aguirunner.WithAppName("edith"),
    aguirunner.WithSessionService(sessionService),
)
```

Web 入口用框架原生的 SSE Service（不包装），GitHub 入口直接调 AG-UI Runner。

---

## 5. 框架在 EDITH 中的典型用法

### 5.1 Agent 装配

- **根 Agent**：LLMAgent
- **多 Agent 协作**：通过 `WithSubAgents` + `transfer_to_xxx` 工具
- **工具**：FunctionTool（GitHub API 封装）+ AgentTool（专家子 Agent）

### 5.2 Session 持久化

```go
import (
    "trpc.group/trpc-go/trpc-agent-go/session/sqlite"
)

sessionService, err := sqlite.NewService(
    sqlite.WithDBPath("./data/edith.db"),
    sqlite.WithSessionEventLimit(1000),
)
```

EDITH 用 SQLite（本地持久化，6 张表 + 异步 worker），不依赖 Redis。

### 5.3 会话摘要（长会话省 token）

```go
llmAgent := llmagent.New("assistant",
    llmagent.WithAddSessionSummary(true),
    llmagent.WithSyncSummaryIntraRun(true),  // 长 ReAct 循环同步触发
)
```

### 5.4 沙箱执行（E2B）

EDITH 不用 trpc-agent-go 自带的 local / container executor，而是用 **自封装的 E2B SDK**（`github.com/eric642/e2b-go-sdk v0.1.3`）。

**自实现思路**（参考 [08-沙箱与Skill.md](./08-沙箱与Skill.md)）：

1. 实现 `Backend` 接口（`RunProgram` / `Read` / `Write` / `Ls` / `Exists` / `Close`）
2. 包装成 `CodeExecutor`（实现 `ExecuteCode` + `CodeBlockDelimiter`）
3. 通过 `llmagent.WithCodeExecutor(e2bExec)` 注入

⚠️ **Skill 文件需手动上传**：E2B 是远程沙箱，框架不自动同步 Skill 脚本。在 Backend init hook 里手动 `e2b.files.write(skillDir)`。

### 5.5 GitHub API 工具

EDITH 工具层是 `function.NewFunctionTool(...)` 包装的 GitHub API：

- 读 Issue / PR 内容
- 读文件
- 提 PR（创建分支、提交、推 PR）
- 评论
- ...

每个工具内部用 `github.com/google/go-github/v69` 客户端 + Installation Token（短期，不持久化）。

### 5.6 Memory（暂未启用）

MVP 阶段 EDITH 未启用 Memory，长期记忆需求尚不明确。如未来需要：
```go
memoryService := memoryinmemory.NewMemoryService(
    memoryinmemory.WithExtractor(extractor.NewExtractor(model)),
)
runner.WithMemoryService(memoryService)
```

### 5.7 Artifact（暂未启用）

EDITH 的沙箱产物由 E2B 的 volume 天然持久化，不需要 Artifact Service。

---

## 6. AG-UI 历史消息统一

### 6.1 框架的限制

`/history` 路由从 `Session.Tracks` 重放，只支持从 AG-UI 自身写入的 track events 读，无法从外部数据库读历史对话。

### 6.2 EDITH 的解法

**Web 与 GitHub 共用同一 AG-UI Runner**（同 appName + userID + sessionID），所有事件都走同一条 pipeline，最终都写入同一份 track。这样：

- Web 发起 Run → 事件写 track
- GitHub 触发 Run → 也写同一 track
- `/history` 重放 track → 看到完整对话

### 6.3 历史会话列表

框架没有内置 HTTP 路由，但 `SessionService.ListSessions` 已支持。MVP 不暴露侧边栏列表，未来需要时：

```text
GET /api/sessions
    ↓
Clerk 鉴权 → UserID
    ↓
SessionService.ListSessions(appName="edith", userKey=UserID)
```

---

## 7. 常用 RunOption（EDITH 场景）

### 7.1 GitHub Webhook 触发

```go
eventCh, _ := sharedAGUIRunner.Run(ctx,
    userID,           // Clerk user_id
    sessionID,        // "github:owner/repo#123"
    model.NewUserMessage(triggerText),
    agent.WithRequestID(runID),
    agent.WithAppName("edith"),
)
```

后台 goroutine 跑完即可，无需等待前端。

### 7.2 多轮 GitHub 评论追加

EDITH 在 GitHub Issue 评论里被多次 @ 时，需要在已有 Session 继续：

- AG-UI 的 `threadId` 用 `sessionID = "github:owner/repo#123"` 固定
- 每次 Run 同一个 sessionID，自动续上
- 用 `runner.EnqueueUserMessage` 在同一 Run 中插队，或新开一个 Run

### 7.3 取消运行

GitHub 触发后用户改主意了（比如改 Issue 标题）：

```go
mr := coreRunner.(runner.ManagedRunner)
mr.Cancel(runID)
```

---

## 8. 常见坑

| 坑 | 现象 | 解法 |
|---|---|---|
| **AG-UI 与 core 版本不一致** | 编译通过但行为不一致 | go.mod 里两个版本同时升级测试 |
| **History 路由读不到 DB 历史** | `/history` 只返回 AG-UI 写入的 track | 确保所有路径（Web + GitHub）都通过共享 AG-UI Runner |
| **E2B Skill 脚本找不到** | `workspace_exec` 找不到 skill 文件 | Backend init hook 里手动上传 |
| **E2B Session 复用失败** | 每次新建 sandbox 慢且贵 | 实现 `e2b.Sandbox.Reconnect` + Session 池 |
| **GitHub API 限流** | 429 Too Many Requests | 用 `cenkalti/backoff` 自动退避 + 缓存 |
| **流式 chunk 写不进 Session** | 想看完整对话回放 | Partial 事件**就是**不写 Session，是正常的 |
| **长会话撑爆 token** | 报错 / 截断 | `WithAddSessionSummary(true)` + `WithMaxToolIterations` |
| **Runner 拥有 inmemory Session 不 Close** | goroutine 泄漏 | EDITH 用 sqlite.SessionService 不是 inmemory，但要 `defer r.Close()` |
| **Webhook 验签失败** | 401 | `golang-jwt/v5` 校验 `X-Hub-Signature-256` |
| **并发同一 SessionKey** | 409 | 默认就有限制；EDITH 同一 Issue 应串行 |
| **多 Agent 串行跑慢** | 步骤多 | `WithConcurrentMessageStreamsEnabled` |
| **Transfer 提示语刷屏前端** | UI 全是"Handing off..." | UI 层过滤 `Object == "agent.transfer"` |

---

## 9. 已验证的子项目

EDITH 在 `forward/` 目录下有一系列验证性小项目（用于验证某个能力）：

| 目录 | 验证内容 |
|---|---|
| `verify_agui` | AG-UI 统一历史（Web + GitHub 同 MessagesSnapshot） |
| `verify_agui_enqueue` | AG-UI Run 中插队用户消息 |
| `verify_core_enqueue_agui` | Core Runner 的 EnqueueUserMessage 通过 AG-UI 出口 |
| `verify_e2b_session_reuse` | E2B Sandbox Session 复用 |
| `verify_enqueue` | Core Runner 的 EnqueueUserMessage 基本能力 |

读这些的 main.go 可以学到 EDITH 真实用法。

---

## 10. 框架能力对应表（EDITH 视角）

| EDITH 需求 | 用 trpc-agent-go 什么 |
|---|---|
| 调 LLM | `openai.New("deepseek-chat")` |
| Agent 定义 | `llmagent.New(...)` |
| 工具（GitHub API） | `function.NewFunctionTool(...)` |
| 子 Agent（专家） | `agenttool.NewTool(agent)` |
| 多 Agent 协作 | `WithSubAgents` + transfer |
| 跑对话 | `runner.NewRunner(...).Run(...)` |
| 会话持久化 | `sqlite.NewService(...)` |
| 长期记忆（未来） | `memoryinmemory.NewMemoryService(...)` |
| Web 实时推 | `aguirunner.New(...)` |
| 历史重放 | `WithMessageSnapshotEnabled(true)` |
| 取消 | `ManagedRunner.Cancel(requestID)` |
| 沙箱执行 | `WithCodeExecutor(e2bExec)` 自定义 |
| 安全审批（未来） | `plugin.NewGuardrail()` |
| 长会话省 token | `WithAddSessionSummary(true)` |
| 思考模式（DeepSeek） | `WithReasoningContentMode(DiscardPreviousTurns)` |
| 多轮上下文 | 同一 SessionID 多次 Run |
| 资源清理 | `defer r.Close()` |

---

## 11. 推荐阅读顺序（EDITH 新人）

1. 本目录 [README.md](./README.md) — 框架速查
2. 本文件 — EDITH 怎么用框架
3. `docs/EDITH/架构设计.md` — EDITH 整体架构
4. `docs/EDITH/细节设计.md` — 入口模型与数据流
5. `docs/EDITH/产品分析.md` — 产品背景
6. `docs/EDITH/MVP实施计划.md` — 实施进度
7. `forward/verify_agui/验证报告.md` — 关键能力验证报告
8. 各 Agent / Tool 源码

---

## 12. 去哪查

- **EDITH 项目文档**：`docs/EDITH/`
- **EDITH 真实代码**：`forward/` 下各子项目
- **go.mod 验证版本**：`go.mod` + `go.sum`
- **框架官方文档**：见 [README.md#6-文档索引](./README.md#6-文档索引按需深读)
