# 03 - Agent 详解

> Agent = "能干什么的单元"。LLMAgent 是主力，多 Agent 组合应对复杂场景。

---

## 1. Agent 接口（5 个方法）

```go
type Agent interface {
    Run(ctx, *Invocation) (<-chan *event.Event, error)
    Tools() []tool.Tool
    Info() Info
    SubAgents() []Agent
    FindSubAgent(name string) Agent
}
```

**注意**：业务代码**不要直接调** `Agent.Run()`，用 `Runner.Run()` 入口。

---

## 2. 五种 Agent 类型

| 类型 | 模式 | 适用场景 | EDITH 用途 |
|---|---|---|---|
| **LLMAgent** | while 循环 + 工具调用 | 通用对话 / 工具调用 | ✅ 主要使用 |
| **ChainAgent** | 串行 1→2→3 | 流水线（分析→研究→写作） | 可选 |
| **ParallelAgent** | 并行多专家 | 多维度评估 | 可选 |
| **CycleAgent** | 迭代直到收敛 | 自改进 / 自动调优 | 可选 |
| **GraphAgent** | 图编排 + 条件分支 | 复杂流程 / HITL 审批 / 暂停恢复 | 未来流程编排 |

**核心区别**：
- **LLMAgent** 控制粒度 = Session 消息级（只能在消息之间拦截）
- **GraphAgent** 控制粒度 = 每个节点（可任意节点精确暂停/恢复）

---

## 3. LLMAgent 配置

### 3.1 基础

```go
agent := llmagent.New("assistant",
    llmagent.WithModel(modelInstance),
    llmagent.WithDescription("A helpful AI assistant"),
    llmagent.WithInstruction("Be helpful, concise, and informative"),
    llmagent.WithGenerationConfig(model.GenerationConfig{
        Stream: true,
        MaxTokens: ptr(2000),
        Temperature: ptr(0.7),
    }),
)
```

### 3.2 工具与多 Agent

```go
agent := llmagent.New("coordinator",
    llmagent.WithTools([]tool.Tool{tool1, tool2}),
    llmagent.WithToolSets([]tool.ToolSet{mcpToolSet}),
    llmagent.WithSubAgents([]agent.Agent{mathAgent, weatherAgent}),
)
```

### 3.3 占位符（动态上下文注入）

`Instruction` 支持的占位符：
- `{key}` — 从 Session.State 读
- `{key?}` — 可选，缺失则替换为空
- `{user:subkey}` — 用户态（跨会话）
- `{app:subkey}` — 应用态
- `{temp:subkey}` — 会话临时（不持久化）
- `{invocation:subkey}` — Invocation.State

```go
llm := llmagent.New("research-agent",
    llmagent.WithInstruction(
        "Focus: {research_topics}. " +
        "User interests: {user:topics?}. " +
        "App banner: {app:banner?}.",
    ),
)
```

**EDITH 用法**：项目配置 / 用户偏好 → 写到 user:*/app:* → 通过占位符注入。

### 3.4 高级选项

| Option | 作用 |
|---|---|
| `WithGlobalInstruction(s)` | 系统提示词前缀（稳定） |
| `WithInstruction(s)` | Agent 指令（追加到 system） |
| `WithModel(m)` / `WithModels(map)` | 模型 / 多模型切换 |
| `WithModelInstructions(map)` / `WithModelGlobalInstructions(map)` | 按模型名覆盖 prompt |
| `WithTools([]Tool)` | 工具列表 |
| `WithToolSets([]ToolSet)` | 工具集（如 MCP） |
| `WithSubAgents([]Agent)` | 子 Agent（可被 transfer_to_xxx 调用） |
| `WithCodeExecutor(exec)` | 代码执行器（沙箱） |
| `WithSkills(repo)` | Skill 仓库 |
| `WithKnowledge(kb)` | 知识库（自动加 knowledge_search 工具） |
| `WithPlanner(p)` | 规划器（Builtin / React / 自定义） |

### 3.5 消息过滤

| Option | 默认 | 说明 |
|---|---|---|
| `WithMessageFilterMode(Mode)` | `FullContext` | 消息可见性（4 模式） |
| `WithMessageTimelineFilterMode(Mode)` | `TimelineFilterAll` | 时间维度 |
| `WithMessageBranchFilterMode(Mode)` | `BranchFilterModePrefix` | 分支维度 |

4 种 FilterMode：
- `FullContext` — 全部可见（含历史）
- `RequestContext` — 仅当前 Run 周期内
- `IsolatedRequest` — 仅当前 Run，自己可见
- `IsolatedInvocation` — 仅当前 Invocation

### 3.6 安全与限制

| Option | 触发后行为 |
|---|---|
| `WithMaxLLMCalls(n)` | 返回 StopError |
| `WithMaxToolIterations(n)` | 发 flow_error 事件，不抛 StopError |
| `WithToolCallRetryPolicy(p)` | 工具失败自动重试（仅 CallableTool） |
| `WithEnablePostToolPrompt(false)` | 关闭工具后提示词注入 |

### 3.7 推理与历史

| Option | 用途 |
|---|---|
| `WithReasoningContentMode(DiscardPreviousTurns)` | DeepSeek 思考模式推荐 |
| `WithToolTranscriptMode(OmitPreviousCompleted)` | 长会话省 token |
| `WithAddSessionSummary(true)` | 摘要注入上下文 |
| `WithSyncSummaryIntraRun(true)` | 同 Run 内同步触发摘要 |
| `WithPreloadMemory(N)` | 记忆自动注入 System Prompt |

### 3.8 结构化输出

| Option | 类型 | 是否允许工具 |
|---|---|---|
| `WithStructuredOutputJSONSchema(name, schema, strict, desc)` | 非类型化 | ✅ 推荐 |
| `WithStructuredOutputJSON[T](...)` | 类型化（Go 结构体） | ✅ 推荐 |
| `WithOutputSchema(schema)` | 非类型化 | ❌ 遗留 |
| `WithOutputKey("key")` | 字符串 | ✅ 写到 Session State 给下游 Agent |

---

## 4. 多 Agent 编排

### 4.1 SubAgent + Transfer

```go
mathAgent := llmagent.New("math",
    llmagent.WithInstruction("你是数学专家"),
    llmagent.WithTools([]tool.Tool{calcTool}),
)

weatherAgent := llmagent.New("weather",
    llmagent.WithInstruction("你是天气专家"),
    llmagent.WithTools([]tool.Tool{weatherTool}),
)

coordinator := llmagent.New("coordinator",
    llmagent.WithSubAgents([]agent.Agent{mathAgent, weatherAgent}),
    llmagent.WithDefaultTransferMessage("Handing off to specialist"),
)
```

**机制**：父 Agent 看到 `transfer_to_math` / `transfer_to_weather` 工具，调用后产生 `agent.transfer` 事件（`Object="agent.transfer"`），前端可按 Tag 过滤。

### 4.2 让追问消息回到原 Agent

```go
r := runner.NewRunner("crm-app", coordinatorAgent,
    runner.WithAwaitUserReplyRouting(true),
)

// LLMAgent 侧
mathAgent := llmagent.New("math",
    llmagent.WithAwaitUserReplyTool(true),  // 暴露 await_user_reply 工具
)
```

**机制**：路由以稳定 Agent 路径存 session state，下一条用户消息自动回到这个 Agent，只消费一次。

### 4.3 Chain / Parallel / Cycle / Graph

```go
// Chain: 串行
chain := chainagent.New("pipeline",
    chainagent.WithSubAgents([]agent.Agent{plan, research, write}),
)

// Parallel: 并行
panel := parallelagent.New("experts",
    parallelagent.WithSubAgents([]agent.Agent{expert1, expert2, expert3}),
)

// Cycle: 迭代
cycle := cycleagent.New("solver",
    cycleagent.WithSubAgents([]agent.Agent{generator, reviewer}),
    cycleagent.WithMaxIterations(5),
)

// Graph: 图编排
stateGraph := graph.NewStateGraph(graph.MessagesStateSchema())
stateGraph.
    AddNode("preprocess", preprocessFn).
    AddLLMNode("analyze", model, "用 analyze 工具", tools).
    AddToolsNode("tools", tools).
    AddConditionalEdges("analyze", condition, map[string]string{
        "simple": "enhance", "complex": "summarize",
    }).
    SetEntryPoint("preprocess").
    Compile()

graphAgent, _ := graphagent.New("processor", stateGraph)
```

---

## 5. Invocation（高级用法）

**正常情况下不要直接用**，但需要完全控制时：

```go
inv := agent.NewInvocation(
    agent.WithInvocationAgent(ag),
    agent.WithInvocationSession(sess),
    agent.WithInvocationMessage(model.NewUserMessage("...")),
    agent.WithInvocationModel(modelInstance),
)

eventCh, err := ag.Run(ctx, inv)
```

**Invocation 结构体关键字段**：
```go
type Invocation struct {
    Agent             Agent
    AgentName         string
    InvocationID      string         // 唯一标识
    Branch            string         // 多 Agent 调用链
    EndInvocation     bool
    Session           *session.Session
    Model             model.Model
    Message           model.Message
    TransferInfo      *TransferInfo  // Agent 转移目标
    StructuredOutput  *model.StructuredOutput
    MemoryService     memory.Service
    ArtifactService   artifact.Service
    MaxLLMCalls       int
    MaxToolIterations int
    Plugins           PluginManager
    // Invocation State（线程安全 K-V）
}
```

**Invocation.State**（跨回调传数据）：
```go
// BeforeAgent 存
args.Invocation.SetState("agent:start_time", time.Now())

// AfterAgent 取
if t, ok := args.Invocation.GetState("agent:start_time"); ok {
    duration := time.Since(t.(time.Time))
}
```

建议命名前缀：`agent:xxx` / `model:xxx` / `tool:<name>:<id>:xxx`。

---

## 6. 动态更新 Instruction（线程安全）

```go
// 运行时改（不重建 Agent）
llm.SetInstruction("Translate all user inputs to French.")
llm.SetGlobalInstruction("System: Safety first. No PII leakage.")

// 下次 Run 用新值
```

---

## 7. 结构导出与按 nodeID 覆盖

### 7.1 静态结构导出

```go
import "trpc.group/trpc-go/trpc-agent-go/agent/structure"

snapshot, _ := structure.Export(ctx, llmAgent)
fmt.Println(snapshot.StructureID)
fmt.Println(snapshot.EntryNodeID)
fmt.Println(len(snapshot.Nodes), len(snapshot.Edges))
```

适合：结构检查、可视化、配置工具。

### 7.2 按 nodeID 覆盖 surface

```go
snapshot, _ := structure.Export(ctx, workflowAgent)
nodeID := findNodeID(snapshot, "planner")

var patch agent.SurfacePatch
patch.SetInstruction("Plan in at most three steps.")
patch.AppendTools([]tool.Tool{priceTool})

events, _ := r.Run(ctx, userID, sessionID, msg,
    agent.WithSurfacePatchForNode(nodeID, patch),
)
```

**支持的 surface**：`SetInstruction` / `SetGlobalInstruction` / `SetFewShot` / `SetModel` / `SetTools` / `AppendTools` / `SetSkillRepository`

**支持的节点**：
- 根 LLMAgent：全部 surface
- graph LLM 节点：instruction/few_shot/model/tool
- graph Tools 节点：tool
- chain/parallel/cycle 子节点：看具体节点支持

### 7.3 执行图追踪

```go
events, _ := r.Run(ctx, userID, sessionID, msg,
    agent.WithExecutionTraceEnabled(true),
)

for ev := range events {
    if ev.IsRunnerCompletion() && ev.ExecutionTrace != nil {
        fmt.Println(ev.ExecutionTrace.RootAgentName)
        fmt.Println(len(ev.ExecutionTrace.Steps))
        for _, step := range ev.ExecutionTrace.Steps {
            fmt.Println(step.NodeID, step.Error)
        }
    }
}
```

挂在 runner completion event 上，每个 Step 带 `NodeID` / `PredecessorStepIDs` / `Input` / `Output` / `Error`。

---

## 8. 踩坑提醒

| 坑 | 解法 |
|---|---|
| 不显式设 `Stream: true` → 默认非流式 | 创建时 `Stream: true` 或 `agent.WithStream(true)` |
| Instruction 占位符 `{key}` 缺失 | 默认保留原样（让 LLM 感知缺失），用 `{key?}` 容忍 |
| 用 transfer 提示语刷屏前端 | UI 层按 `Tag` / `Object == "agent.transfer"` 过滤 |
| `WithMaxLLMCalls` 太紧 | 区分 MaxLLMCalls（抛 StopError）和 MaxToolIterations（不发 StopError） |
| 删了 `Skill.load` 但开 `WithSkills` | 加 `WithSkills(repo)` 没显式 executor 时自动注入本地 executor |
| GraphAgent 中途退出读不到结果 | 用 `IsRunnerCompletion()` + `StateDelta[graph.StateKeyLastResponse]` |

---

## 9. 去哪查

- **官方详细**：`docs/trpc-agent-go/docs/mkdocs/zh/agent.md`
- **多 Agent**：`docs/trpc-agent-go/docs/mkdocs/zh/multiagent.md`
- **Graph**：`docs/trpc-agent-go/docs/mkdocs/zh/graph.md`
- **结构导出**：`docs/trpc-agent-go/docs/mkdocs/zh/agent.md#静态结构导出`
