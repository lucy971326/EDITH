# 04 - Tool 与 MCP

> Tool = Agent 能调用的能力。三种实现：本地函数 / MCP 远端 / 子 Agent。

---

## 1. 接口族

```go
// 最基础：只描述工具是什么
type Tool interface {
    Declaration() *Declaration
}

// 可同步调用
type CallableTool interface {
    Call(ctx context.Context, jsonArgs []byte) (any, error)
    Tool
}

// 可流式调用
type StreamableTool interface {
    StreamableCall(ctx context.Context, jsonArgs []byte) (*tool.StreamReader, error)
    Tool
}

// 工具集：一组工具的容器
type ToolSet interface {
    Tools(context.Context) []Tool
    Close() error
    Name() string
}
```

**三种实现的核心区别**：

```
FunctionTool.Call()   → 跑本地函数
MCP Tool.Call()       → HTTP 调远端 MCP 服务器
AgentTool.Call()      → 起个子 Agent 干活

Declaration() 都是一样的：告诉 LLM 工具叫什么、要什么参数
```

---

## 2. Declaration（元信息）

LLM 用 `Declaration` 决定要不要调工具 + 怎么传参：

```go
type Declaration struct {
    Name        string
    Description string    // 描述决定 LLM 用不用
    InputSchema *Schema   // JSON Schema
    OutputSchema *Schema  // 返回值 JSON Schema
}
```

---

## 3. FunctionTool（最常用）

### 3.1 基本用法

```go
import "trpc.group/trpc-go/trpc-agent-go/tool/function"

type ReadFileInput struct {
    Path string `json:"path" description:"要读取的文件路径"`
}
type ReadFileOutput struct {
    Content string `json:"content" description:"文件内容"`
}

readFileTool := function.NewFunctionTool(
    func(ctx context.Context, in ReadFileInput) (ReadFileOutput, error) {
        data, err := os.ReadFile(in.Path)
        return ReadFileOutput{Content: string(data)}, err
    },
    function.WithName("read_file"),
    function.WithDescription("读取本地文件内容"),
)

agent := llmagent.New("assistant",
    llmagent.WithTools([]tool.Tool{readFileTool}),
)
```

### 3.2 选项

| Option | 作用 |
|---|---|
| `WithName(s)` | 工具名（LLM 调用时用） |
| `WithDescription(s)` | 工具描述 |
| `WithLongRunning(true)` | 标记为长任务（前端可显示加载状态） |
| `WithSkipSummarization(true)` | 跳过工具后总结 LLM 调用 |
| `WithInputSchema(schema)` | 自定义入参 schema |
| `WithOutputSchema(schema)` | 自定义出参 schema |

### 3.3 流式工具

```go
weatherStreamTool := function.NewStreamableFunctionTool[weatherInput, weatherOutput](
    func(ctx context.Context, in weatherInput) (*tool.StreamReader, error) {
        stream := tool.NewStream(10)
        go func() {
            defer stream.Writer.Close()
            for _, chunk := range chunks {
                stream.Writer.Send(tool.StreamChunk{
                    Content: weatherOutput{Weather: chunk},
                }, nil)
            }
        }()
        return stream.Reader, nil
    },
    function.WithName("get_weather_stream"),
)
```

### 3.4 在 Tool 里拿 Tool Call ID

```go
import "trpc.group/trpc-go/trpc-agent-go/tool"

toolCtx, _ := tool.NewContext(ctx)
callID := toolCtx.ToolCallID  // 当前 LLM 发起的工具调用 ID
```

### 3.5 工具内拉起子 Agent（Tool → Agent）

如果你的 Tool 内部需要起子 Agent（更复杂的工具）：

```go
import "trpc.group/trpc-go/trpc-agent-go/agent"
import agenttool "trpc.group/trpc-go/trpc-agent-go/agent/agenttool"

mathTool := agenttool.NewTool(mathAgent)
result, _ := mathTool.Call(ctx, jsonArgs)
```

### 3.6 工具重试

```go
policy := &tool.RetryPolicy{
    MaxAttempts:     2,
    InitialInterval: 200 * time.Millisecond,
    BackoffFactor:   2.0,
    MaxInterval:     time.Second,
    // RetryOn: func(err error) bool { return ... },  // 自定义重试条件
}

agent := llmagent.New("assistant",
    llmagent.WithTools([]tool.Tool{readFileTool}),
    llmagent.WithToolCallRetryPolicy(policy),
)
```

默认只重试瞬时错误（`io.EOF` / `io.ErrUnexpectedEOF` / 网络超时），可自定义 `RetryOn`。

---

## 4. MCP ToolSet（远程工具）

### 4.1 三种传输

```go
import "trpc.group/trpc-go/trpc-agent-go/tool/mcp"

// 1. STDIO：子进程（本地脚本/CLI）
mcp.NewMCPToolSet(mcp.ConnectionConfig{
    Transport: "stdio",
    Command:   "python",
    Args:      []string{"-m", "my_mcp_server"},
})

// 2. SSE：Server-Sent Events（远程长连接）
mcp.NewMCPToolSet(mcp.ConnectionConfig{
    Transport: "sse",
    ServerURL: "http://localhost:8080/sse",
    Headers:   map[string]string{"Authorization": "Bearer xxx"},
})

// 3. Streamable HTTP：标准 HTTP 流式（简单）
mcp.NewMCPToolSet(mcp.ConnectionConfig{
    Transport: "streamable_http",
    ServerURL: "http://localhost:3000/mcp",
})
```

### 4.2 选项

| Option | 作用 |
|---|---|
| `WithName(s)` | ToolSet 名称 |
| `WithToolFilterFunc(f)` | 工具过滤（如只暴露指定名称） |
| `WithSessionReconnect(n)` | 自动重连（1-10 次） |
| `WithMCPOptions(...)` | 透传底层 MCP 选项 |
| `WithToolSetInitTimeout(d)` | 初始化超时 |

### 4.3 启动预加载

```go
if err := mcpToolSet.Init(ctx); err != nil {
    log.Fatalf("MCP 初始化失败: %v", err)
}
defer mcpToolSet.Close()
```

`Init()` 可选但推荐：启动时预加载工具列表，快速失败。

### 4.4 注册到 Agent

```go
agent := llmagent.New("mcp-assistant",
    llmagent.WithToolSets([]tool.ToolSet{mcpToolSet}),
)
```

---

## 5. AgentTool（Agent 包装成 Tool）

把另一个 Agent 包装成 Tool，让上层 Agent 可以"派出小弟"干活。

### 5.1 两种创建方式

```go
import agenttool "trpc.group/trpc-go/trpc-agent-go/agent/agenttool"

// 方式一：包装一个固定 Agent
mathTool := agenttool.NewTool(
    mathAgent,
    agenttool.WithName("math_expert"),
    agenttool.WithDescription("调用数学专家"),
)

// 方式二：动态 Agent（模型自己选参数）
dynamicAgent := agenttool.NewDynamicTool()
```

### 5.2 选项

| Option | 作用 |
|---|---|
| `WithName(s)` | 工具名 |
| `WithDescription(s)` | 工具描述 |
| `WithSkipSummarization(bool)` | 跳过工具后总结 |
| `WithStreamInner(bool)` | 转发子 Agent 流式事件 |
| `WithInnerTextMode(mode)` | 是否转发子 Agent 正文（`Include`/`Exclude`） |
| `WithResponseMode(mode)` | 工具结果只返回最后一条 |
| `WithHistoryScope(scope)` | 子 Agent 能否看到父历史（`Isolated`/`ParentBranch`） |

### 5.3 历史可见性

- **`Isolated`**：子 Agent 看不到父 Agent 历史（独立任务）
- **`ParentBranch`**：能看到父 Agent 所在分支的消息（协作场景）

---

## 6. 工具元数据与权限策略

详见 `tool.md` 文档的 `Tool Metadata 与权限策略` 章节。

简单示例：
```go
import "trpc.group/trpc-go/trpc-agent-go/tool"

tool.WithToolMeta(myTool, &tool.Metadata{
    Tags:        []string{"file", "read"},
    RequireAuth: true,
})
```

---

## 7. 工具错误处理

```go
result, err := tool.Call(ctx, jsonArgs)
if err != nil {
    // 业务错误：返回 error，会作为 tool response 传给 LLM
    // 框架错误：panic 会被 recover
}
```

LLM 看到 tool result 后会：
- 成功 → 继续推理或返回最终答案
- 失败 → 重试或换其他工具

---

## 8. 踩坑提醒

| 坑 | 解法 |
|---|---|
| 工具描述不清，LLM 不调用 | 描述要说明"什么时候用 + 怎么用" |
| 工具返回结构体没 JSON tag | LLM 拿到的是 JSON，加 `json:"xxx"` tag |
| MCP server 启动慢 | 用 `Init()` 预加载 + 单独超时 |
| MCP 连接断开 | `WithSessionReconnect(n)` 自动重连 |
| AgentTool 内部又调 AgentTool → 死循环 | 限制 `WithMaxToolIterations` |
| Tool 内启动 goroutine 不受 ctx 控制 | 在 goroutine 里 `select { case <-ctx.Done(): }` |
| StreamableTool 没有 Call 方法 | 注册时只能用 CallableTool 才能用重试 |

---

## 9. 去哪查

- **Tool 完整文档**：`docs/trpc-agent-go/docs/mkdocs/zh/tool.md`
  - Function Tools 详细用法：`tool.md#function-tools-函数工具`
  - Tool Metadata 与权限：`tool.md#tool-metadata-与权限策略`
  - ToolSet 与流式：`tool.md#toolset工具集` / `tool.md#流式工具支持`
  - 内置工具（重试/DuckDuckGo/Claude Code）：`tool.md#内置工具类型`
- **AgentTool 详解**：`docs/trpc-agent-go/docs/mkdocs/zh/agent.md`（搜索 "AgentTool"）
- **MCP**：在 `tool.md` 中搜索 "MCP"
