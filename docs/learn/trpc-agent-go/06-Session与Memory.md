# 06 - Session、Memory、Artifact

> 三件套：Session = 单次会话历史，Memory = 跨会话用户档案，Artifact = 文件柜。

---

## 1. Session（会话管理）

### 1.1 心智模型

> Session = Agent 的"对话记忆本"，自动保存、加载、压缩对话历史。
> 隔离维度 `<AppName, UserID, SessionID>`，支持多种存储后端，自动管理多轮对话上下文。

### 1.2 核心结构

```go
type Key struct {
    AppName   string
    UserID    string
    SessionID string
}

type Session struct {
    ID         string
    AppName    string
    UserID     string
    State      StateMap                    // 会话状态（支持 delta）
    Events     []event.Event               // 事件列表
    Tracks     map[Track]*TrackEvents      // Track 事件（AG-UI 用）
    Summaries  map[string]*Summary         // 摘要列表（filterKey → Summary）
    UpdatedAt  time.Time
    CreatedAt  time.Time
    Hash       uint32
    ServiceMeta map[string]string
}
```

### 1.3 Service 接口（14 方法）

```go
type Service interface {
    // 会话 CRUD
    CreateSession(ctx, key, state, opts...) (*Session, error)
    GetSession(ctx, key, opts...) (*Session, error)
    ListSessions(ctx, userKey, opts...) ([]*Session, error)
    DeleteSession(ctx, key, opts...) error

    // 事件追加
    AppendEvent(ctx, session, event, opts...) error

    // 状态管理
    UpdateAppState(ctx, appName, state) error
    DeleteAppState(ctx, appName, key) error
    ListAppStates(ctx, appName) (StateMap, error)
    UpdateUserState(ctx, userKey, state) error
    ListUserStates(ctx, userKey) (StateMap, error)
    DeleteUserState(ctx, userKey, key) error
    UpdateSessionState(ctx, key, state) error

    // 摘要
    CreateSessionSummary(ctx, sess, filterKey, force) error
    EnqueueSummaryJob(ctx, sess, filterKey, force) error
    GetSessionSummaryText(ctx, sess, opts...) (string, bool)

    Close() error
}
```

### 1.4 八种存储后端

| 后端 | 适用场景 | 文档 |
|---|---|---|
| **Memory（内存）** | 开发测试 / 小规模 | `session/inmemory.md` |
| **SQLite** | 本地持久化 / 单机 | `session/sqlite.md` |
| **Redis** | 生产 / 分布式 | `session/redis.md` |
| **PostgreSQL** | 生产 / 复杂查询 | `session/postgres.md` |
| **MySQL** | 生产 / 软删除 | `session/mysql.md` |
| **ClickHouse** | 海量日志 / 分析 | `session/clickhouse.md` |
| **MongoDB** | 文档型存储 | `session/mongodb.md` |
| **TDSQL** | 腾讯 TDSQL | `session/tdsql.md` |

### 1.5 关键写入规则

- **只有 `!IsPartial && IsValidContent()` 才写 Session**（流式 chunk 不持久化）
- 每次写入后自动触发 `EnqueueSummaryJob`（异步摘要检查）
- SQLite 用 `stateWriteMu` 串行化写
- Redis 用 `HashIdx`（新，默认，按 userID 散列）或 `ZSet`（旧，兼容）

### 1.6 Runner 集成（9 个调用点）

```
Runner.Run()
  ├─ ① getOrCreateSession()
  ├─ ② appendMessagesAsSessionEvents()    ← 种子消息
  ├─ ③ appendIncomingMessage()            ← 用户输入
  ├─ ④ persistEvent()                     ← 助手回复/工具调用
  ├─ ⑤ persistAgentRunError()
  ├─ ⑥ EnqueueSummaryJob()
  ├─ ⑦ AppendEvent(completion)            ← 最终完成
  ├─ ⑧ EnqueueSummaryJob()
  └─ ⑨ runner.Close() → sessionService.Close()
```

---

## 2. 会话摘要（Summary）

### 2.1 心智模型

> 摘要 = Session 的"省流助手"—— 用 LLM 把长对话浓缩成剧情简介，保留关键上下文同时省 token。

### 2.2 三件套配置

```go
// ① 摘要器
summarizer := summary.NewSummarizer(
    summaryModel,
    summary.WithChecksAny(
        summary.CheckEventThreshold(20),     // 事件数阈值
        summary.CheckTokenThreshold(4000),   // token 阈值
        summary.CheckTimeThreshold(5*time.Minute),
    ),
    summary.WithMaxSummaryWords(200),
)

// ② Session Service 注入摘要器
sessionService := inmemory.NewSessionService(
    inmemory.WithSummarizer(summarizer),
    inmemory.WithAsyncSummaryNum(2),
    inmemory.WithSummaryQueueSize(100),
)

// ③ Agent 启用摘要注入
llmAgent := llmagent.New("assistant",
    llmagent.WithAddSessionSummary(true),
    llmagent.WithSyncSummaryIntraRun(true),  // 长 ReAct 循环同步触发
)
```

### 2.3 触发条件

| 条件 | 推荐场景 |
|---|---|
| `CheckEventThreshold(N)` | 稳定轮次间隔 |
| `CheckTokenThreshold(N)` | token 预算敏感 |
| `CheckContextThreshold()` | **推荐**：自动适配当前模型 context window |
| `CheckTimeThreshold(d)` | 兜底防静默期 |

### 2.4 增量摘要（核心机制）

不是每次都把全部事件扔给 LLM！只处理增量：

```
首次: [e1...e20] → LLM → 摘要A  (记录边界 last_event_id=e20)

第2次: 摘要A + [e21...e40] → LLM → 摘要B
第3次: 摘要B + [e41...] → LLM → 摘要C
```

**boundary 决定"增量边界"**：`computeDeltaAfterBoundary` 通过 `LastEventID` 找新事件，`prependPrevSummary` 把旧摘要作为合成系统事件前置。

并发安全：`<appName, userID, sessionID, filterKey>` 二元信号量锁。

### 2.5 两种上下文模式

| 模式 | 配置 | 适用 |
|---|---|---|
| **摘要注入**（推荐） | `WithAddSessionSummary(true)` | 长期会话，摘要+增量 |
| **截断模式** | `WithMaxHistoryRuns(N)` | 短会话/测试 |

完整文档：`session/summary.md`

---

## 3. Memory（长期记忆）

### 3.1 心智模型

> Memory = Agent 的"用户档案"—— 跨会话记住用户是谁，不依赖特定对话历史。
> 隔离维度 `<AppName, UserID>`（跨会话），与 Session 的 `<AppName, UserID, SessionID>` 不同层级。

### 3.2 Memory vs Session

| | Memory | Session |
|---|---|---|
| 隔离维度 | `<AppName, UserID>` | `<AppName, UserID, SessionID>` |
| 生命周期 | **跨会话持久** | 单次会话 |
| 存什么 | 用户画像、偏好、事实 | 对话历史、消息记录 |
| 数据量 | 小（几十~几百条） | 大（几十~几千条） |

### 3.3 Service 接口（9 方法）

```go
type Service interface {
    AddMemory(ctx, userKey, memory, topics, opts...) error
    UpdateMemory(ctx, memoryKey, memory, topics, opts...) error
    DeleteMemory(ctx, memoryKey) error
    ClearMemories(ctx, userKey) error
    ReadMemories(ctx, userKey, limit) ([]*Entry, error)
    SearchMemories(ctx, userKey, query, opts...) ([]*Entry, error)
    Tools() []tool.Tool              // 暴露给 Agent 的工具
    EnqueueAutoMemoryJob(ctx, sess) error
    Close() error
}
```

### 3.4 两种模式

**Agentic（工具驱动）** — Agent 主动调：
```go
memoryService := memoryinmemory.NewMemoryService()

llmAgent := llmagent.New("assistant",
    llmagent.WithTools(memoryService.Tools()),
)
// 默认暴露 add/update/search/load，delete/clear 需显式启用
```

**Auto（自动提取，推荐）** — 后台 LLM 静默提取：
```go
extractor := extractor.NewExtractor(model)
memoryService := memoryinmemory.NewMemoryService(
    memoryinmemory.WithExtractor(extractor),
    memoryinmemory.WithAsyncMemoryNum(1),
)

llmAgent := llmagent.New("assistant",
    llmagent.WithTools(memoryService.Tools()),
)
// 默认只暴露 search，写操作在后台
```

### 3.5 6 个工具

| 工具 | Agentic 默认 | Auto 默认 |
|---|---|---|
| `memory_add` | ✅ | ❌（后台用） |
| `memory_update` | ✅ | ❌（后台用） |
| `memory_search` | ✅ | ✅ |
| `memory_load` | ✅ | ⚙️ 需启用 |
| `memory_delete` | ⚙️ 需启用 | ❌ |
| `memory_clear` | ⚙️ 需启用 | ❌ 禁用 |

### 3.6 Memory Preload

```go
llmAgent := llmagent.New("assistant",
    llmagent.WithPreloadMemory(10),
)
```

行为：
- `0`：禁用
- `N > 0`：自适应（≤N 全量 / >N 搜 topN）
- `-1`：全量（⚠️ token 风险）

效果：自动把相关记忆合并到 System Prompt。

### 3.7 幂等 ID

```go
// SHA256("memory:内容|app:应用名|user:用户ID[|kind:kind][|event_time:时间]")
// Topics 不参与 ID，改主题不会新建记忆
```

### 3.8 7 种后端

| 后端 | 持久化 | 向量搜索 |
|---|---|---|
| InMemory | ❌ | ❌ |
| SQLite | ✅ | ❌ |
| SQLiteVec | ✅ | ✅ |
| Redis | ✅ | ❌ |
| MySQL | ✅ | ❌ |
| PostgreSQL | ✅ | ❌ |
| pgvector | ✅ | ✅ |
| MySQL Vector | ✅ | ✅ |

完整文档：`memory.md`

---

## 4. Artifact（制品）

### 4.1 心智模型

> Artifact = Agent 的"文件柜"—— 存图片、文档、二进制数据，版本化管理。
> 通过 `toolCtx.SaveArtifact()` 保存文件，`toolCtx.LoadArtifact()` 加载文件。

### 4.2 Service 接口（5 方法）

```go
type Service interface {
    SaveArtifact(ctx, sessionInfo, filename, artifact) (int, error)  // 返回版本号
    LoadArtifact(ctx, sessionInfo, filename, version) (*Artifact, error)  // nil=最新版
    ListArtifactKeys(ctx, sessionInfo) ([]string, error)
    DeleteArtifact(ctx, sessionInfo, filename) error
    ListVersions(ctx, sessionInfo, filename) ([]int, error)
}
```

### 4.3 命名空间与版本

```
存文件时：
  toolCtx.SaveArtifact("report.pdf", artifact)
    → 存到: {app}/{user}/{session}/report.pdf/{version}
    → 版本自动递增：v0 → v1 → v2

跨会话持久化（加 user: 前缀）：
  toolCtx.SaveArtifact("user:profile.json", artifact)
    → 存到: {app}/{user}/user/profile.json/{version}
    → 所有会话都能读到

读取时指定版本：
  toolCtx.LoadArtifact("report.pdf", &v0)  // 读第 0 版
  toolCtx.LoadArtifact("report.pdf", nil)  // 读最新版
```

### 4.4 工具里读写

```go
func myTool(ctx context.Context, input Input) (Output, error) {
    toolCtx, _ := agent.NewToolContext(ctx)

    // 保存文件
    version, _ := toolCtx.SaveArtifact("greeting.txt", &artifact.Artifact{
        Data:     []byte("Hello World!"),
        MimeType: "text/plain",
    })

    // 加载文件
    loaded, _ := toolCtx.LoadArtifact("greeting.txt", nil)
    content := string(loaded.Data)

    return output, nil
}
```

### 4.5 三种后端

| 后端 | 用途 |
|---|---|
| InMemory | 开发测试 |
| S3（兼容 MinIO/R2） | AWS / 自部署 |
| COS | 腾讯云 |

完整文档：`artifact.md`

---

## 5. 何时用哪个

| 场景 | 用什么 |
|---|---|
| 用户说了什么 | Session |
| 用户是谁 / 偏好 / 事实 | Memory |
| 工具生成的文件 | Artifact |
| Session 越来越长 | Summary + Memory |
| 跨会话查用户偏好 | Memory.Preload |

---

## 6. 组合用法示例

```go
// 三件套全配置
memoryService := memoryinmemory.NewMemoryService(
    memoryinmemory.WithExtractor(extractor.NewExtractor(model)),
)
artifactService := inmemory.NewService()

sessionService := inmemory.NewSessionService(
    inmemory.WithSummarizer(summarizer),
)

agent := llmagent.New("assistant",
    llmagent.WithModel(model),
    llmagent.WithTools(memoryService.Tools()),
    llmagent.WithAddSessionSummary(true),
    llmagent.WithPreloadMemory(20),
)

r := runner.NewRunner("my-app", agent,
    runner.WithSessionService(sessionService),
    runner.WithMemoryService(memoryService),
    runner.WithArtifactService(artifactService),
)
```

---

## 7. 踩坑提醒

| 坑 | 解法 |
|---|---|
| 流式 chunk 写不进 Session | 这是正常的，Partial 事件不持久化 |
| 摘要消耗太多 token | 用 `CheckContextThreshold` 自动适配 |
| Memory 重复创建 | 相同内容 SHA256 ID 相同，会覆盖更新 |
| 误删所有记忆 | `memory_clear` 默认禁用，显式启用 |
| Artifact 跨会话找不到 | 加 `user:` 前缀 |
| Session 没 Close → goroutine 泄漏 | 必加 `defer r.Close()`（inmemory 时） |
| Redis 热点 | 用新版 HashIdx（按 userID 散列） |

---

## 8. 去哪查

- **Session 总览**：`docs/trpc-agent-go/docs/mkdocs/zh/session.md`
- **Session 各后端**：`docs/trpc-agent-go/docs/mkdocs/zh/session/<backend>.md`
- **摘要详解**：`docs/trpc-agent-go/docs/mkdocs/zh/session/summary.md`
- **Memory 完整**：`docs/trpc-agent-go/docs/mkdocs/zh/memory.md`
- **Artifact 完整**：`docs/trpc-agent-go/docs/mkdocs/zh/artifact.md`
