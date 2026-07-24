# 02 - Runner 详解

> Runner 是业务代码的唯一入口。"传消息进来，事件流出去"，其他全自动。

---

## 1. 接口定义

```go
type Runner interface {
    Run(ctx, userID, sessionID, message, opts...) (<-chan *event.Event, error)
}

type ManagedRunner interface {
    Runner
    Cancel(requestID)
    RunStatus(requestID) RunStatus
}
```

**两件事**：
1. `Run()` 一次对话
2. `ManagedRunner` 跨 goroutine 取消 / 查状态

---

## 2. Runner 核心职责

```
Runner.Run(ctx, userID, sessionID, msg)
  ├─ ① 生成 / 复用 Session
  ├─ ② 生成 InvocationID + RequestID
  ├─ ③ 拼装 Invocation（Agent + Session + Model + Message + RunOptions + Plugins）
  ├─ ④ Agent.RunWithPlugins(invocation)  ← 内部启动 goroutine
  ├─ ⑤ 事件循环
  │    ├─ Plugins.OnEvent() 拦截
  │    ├─ 非 partial 事件 → AppendEvent 写 Session
  │    └─ 转发到 eventChan
  └─ ⑥ 发送 runner.completion 事件
```

**业务代码完全不用关心**：会话创建、回调组装、事件分发、状态增量合并。

---

## 3. 构造选项

```go
r := runner.NewRunner("app-name", agent,
    runner.WithSessionService(sessionService),  // 默认是 inmemory
    runner.WithMemoryService(memoryService),    // 可选
    runner.WithArtifactService(artService),     // 可选
    runner.WithPlugins(plugin1, plugin2),       // 全局插件
    runner.WithAgent("name2", agent2),          // 注册多个按名选
    runner.WithRalphLoop(runner.RalphLoopConfig{
        MaxIterations:     20,
        CompletionPromise: "DONE",
        VerifyCommand:     "go test ./... -count=1",
    }),
    runner.WithPersistInterruptedAssistant(true), // 中断时持久化已生成的文本
    runner.WithAwaitUserReplyRouting(true),       // 支持多 Agent 追问路由
    runner.WithCandidateSelector(selector),       // Best-of-N 候选选择
)
```

---

## 4. RunOption 速查（每次 Run 临时生效）

### 4.1 标识与路由

| Option | 作用 |
|---|---|
| `agent.WithRequestID("req-123")` | 传入 requestID，配合 ManagedRunner 取消/查状态 |
| `agent.WithAppName("project-a")` | 单 Runner 多租户隔离（session 写到不同 appName 下） |
| `agent.WithAgent(myAgent)` | 单次 Run 临时覆盖 Agent |
| `agent.WithAgentByName("name2")` | 用 `WithAgent` 注册的命名 Agent |
| `agent.WithAgentFactory(...)` | 每次 Run 动态创建 Agent |

### 4.2 取消与超时

| Option | 作用 |
|---|---|
| `agent.WithDetachedCancel(true)` | 父 ctx cancel 不影响本次 run |
| `agent.WithMaxRunDuration(30s)` | 硬截止时长（与 ctx deadline 取较小者） |

### 4.3 消息处理

| Option | 作用 |
|---|---|
| `agent.WithMessages([]Message)` | 上游维护的历史 → auto-seed Session |
| `agent.WithUserMessageRewriter(fn)` | 改写用户消息（可 1→N 展开） |
| `agent.WithResume(true)` | 继续上次未完成的工具调用 |
| `agent.WithToolCallArgumentsJSONRepairEnabled(true)` | 自动修复非严格 JSON |

### 4.4 上下文注入（非持久化）

| Option | 作用 |
|---|---|
| `agent.WithInjectedContextMessages([]Message)` | 注入到 session history **之前**（背景） |
| `agent.WithLateContextMessages([]Message)` | 注入到**贴近最新用户回合**（动态约束） |
| `agent.WithGlobalInstruction(s)` | 覆盖 system prompt 前缀 |
| `agent.WithInstruction(s)` | 覆盖根 Agent instruction |

### 4.5 覆盖与扩展

| Option | 作用 |
|---|---|
| `agent.WithCodeExecutor(exec)` | 单次 Run 临时覆盖 CodeExecutor |
| `agent.WithSurfacePatchForNode(nodeID, patch)` | 按 nodeID 精确覆盖 surface |

### 4.6 输出过滤

| Option | 作用 |
|---|---|
| `agent.WithStreamMode(StreamModeMessages)` | 只转发 messages 事件（隐藏 graph/checkpoint 等） |
| `agent.WithGraphEmitFinalModelResponses(true)` | Graph LLM 节点输出最终 Done=true 消息 |
| `agent.WithGraphTerminalMessagesOnly(true)` | 只保留 terminal 节点消息 |
| `agent.WithPersistInterruptedAssistant(true)` | 中断时持久化已生成文本 |

### 4.7 单次 Run 注入

| Option | 作用 |
|---|---|
| `plugin.WithPlugins(p)` | Run 级插件（排在 Runner 级之后，只本次生效） |

---

## 5. 取消与并发的四种姿势

### 5.1 ctx 取消（最常用）

```go
ctx, cancel := context.WithCancel(context.Background())
defer cancel()

eventCh, _ := r.Run(ctx, userID, sessionID, msg)

// 想停：cancel()
go func() {
    time.Sleep(5 * time.Second)
    cancel()
}()

// 必须读完 channel，否则 goroutine 在写时会阻塞！
for range eventCh {
}
```

### 5.2 requestID 取消（跨 goroutine）

```go
requestID := "req-123"
eventCh, _ := r.Run(ctx, userID, sessionID, msg,
    agent.WithRequestID(requestID))

// 另一个 goroutine
mr := r.(runner.ManagedRunner)
mr.Cancel(requestID)

for range eventCh {}

// 查状态
status, ok := mr.RunStatus(requestID)
```

### 5.3 队列插入用户消息（steer）

```go
go func() {
    time.Sleep(time.Second)
    runner.EnqueueUserMessage(r, requestID,
        model.NewUserMessage("再补充一下"))
}()
```

新消息插在两轮 assistant 之间，**不会**插入到 tool_call 中间。

### 5.4 内部触发停止（StopError）

在工具/回调里：
```go
return agent.NewStopError("超出预算")  // 框架会发 stop_agent_error 事件
```

---

## 6. Ralph Loop（外循环）

```go
r := runner.NewRunner("my-app", a,
    runner.WithRalphLoop(runner.RalphLoopConfig{
        MaxIterations:     20,
        CompletionPromise: "DONE",
        VerifyCommand:     "go test ./... -count=1",
        VerifyTimeout:     2 * time.Minute,
    }),
)
```

**核心思想**：不靠 LLM 主观判断"完成了"，用可验证条件（promise / 测试退出码 / 自定义 Verifier）。`MaxIterations` 是安全阀。

---

## 7. Agent Factory（按请求动态构造）

```go
r := runner.NewRunnerWithAgentFactory("my-app", "default",
    func(ctx context.Context, ro agent.RunOptions) (agent.Agent, error) {
        a := llmagent.New("default",
            llmagent.WithInstruction(ro.Instruction),
        )
        return a, nil
    },
)
```

**重要边界**：
- factory 内部创建的 ToolSet / MCP 连接 / 沙箱**不会**被 Runner.Close() 自动释放
- `agent.Agent` 接口没有 Close，Runner 无法统一接管
- 解决方案：factory 外创建一次 + 复用，或自己包一层带清理的 Agent

---

## 8. 资源管理

**关键**：
- Runner 拥有 `inmemory.NewSessionService()` 时**必须** `Close()`
- 第三方 Session Service 由你 Close（Runner 不管）
- Close 幂等

```go
r := runner.NewRunner("my-app", agent)
defer r.Close()  // 必需！
```

---

## 9. 踩坑提醒

| 坑 | 解法 |
|---|---|
| 只 break 不 cancel → goroutine 泄漏 | 必须 cancel() + 读完 channel |
| `context.Background()` 没法取消 | 用 `context.WithCancel` 或 `WithTimeout` |
| 工具忽略 ctx → 取消不了 | 长耗时工具必须 `select { case <-ctx.Done(): }` |
| Runner 拥有 inmemory Session 不 Close | 必加 `defer r.Close()` |
| 多 Runner 共用 SessionService | 可以，注意并发安全 |
| Agent Factory 里创建 ToolSet 不释放 | factory 外创建 + 复用，或自带 Close |

---

## 10. 去哪查

- **官方详细文档**：`docs/trpc-agent-go/docs/mkdocs/zh/runner.md`
- **事件消费示例**：`docs/trpc-agent-go/docs/mkdocs/zh/runner.md#事件处理`
- **取消与超时示例**：`docs/trpc-agent-go/docs/mkdocs/zh/runner.md#使用注意事项`
- **Best-of-N**：`docs/trpc-agent-go/docs/mkdocs/zh/runner.md#在线-best-of-n-候选选择`
- **远程 Runner（tRPC-Agent API）**：`docs/trpc-agent-go/docs/mkdocs/zh/runner.md#远程-trpc-agent-runner`
