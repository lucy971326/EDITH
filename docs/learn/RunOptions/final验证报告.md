# EDITH 服务端 Agent 验证报告

> 2026-07-27 在 `forward/` 目录完成的端到端验证。所有 RunOptions 决策 + 关键行为都跑通，**EDITH 服务端 Agent 改造方案完全可行**。

---

## 1. 验证目标

把 [EDITH RunOptions 字段决策](../RunOptions/5.EDITH字段决策总表.md) 的 7 个核心字段 + Tool + MCP + ctx 取消机制 + 多租户 MCP 模式，在最小代码里全部跑通。

## 2. 验证结果（全部通过）

| # | 验证项 | 用例 | 结果 |
|---|---|---|---|
| 1 | Model 双 Variant（DeepSeek + MiniMax） | 全部 | ✅ |
| 2 | Agent + Runner 骨架 | 全部 | ✅ |
| 3 | `WithRequestID` | 全部 | ✅ UUID 链路追踪 |
| 4 | `WithStream` | 全部 | ✅ 流式分片 |
| 5 | `WithModelName` | 用例 1/2/3 | ✅ deepseek ↔ minimax 切换 |
| 6 | `WithModelRequestHeaders` | 全部 | ✅ Authorization 多租户 |
| 7 | `WithGlobalInstruction` | 全部 | ✅ L1 身份覆盖（暴躁 vs 温和） |
| 8 | `WithInstruction` | 全部 | ✅ L3 skills 覆盖 |
| 9 | `WithAdditionalTools` | 用例 2/3 | ✅ Tavily MCP 远程搜索 |
| 10 | FunctionTool 调用 | 用例 1（删 GetTime 后） | ✅ GetTime + LLM 主动调 |
| 11 | MCP ToolSet 远程调用 | 用例 2/3 | ✅ Tavily `tavily-search` |
| 12 | ctx 取消机制 | 用例 3（2.5s 超时） | ✅ `context deadline exceeded` 跨层传播 |
| 13 | **多租户 MCP（每个用户独立 ToolSet）** | 用例 2/3 | ✅ alice/bob 各自带 key |

## 3. 关键发现

### 3.1 事件循环正确写法

```go
// ❌ 错：流式场景会重复打印
if choice.Delta.Content != "" { fmt.Print(choice.Delta.Content) }
if choice.Message.Content != "" { fmt.Print(choice.Message.Content) }

// ✅ 对：流式只读 Delta
if choice.Delta.Content != "" { fmt.Print(choice.Delta.Content) }

// ✅ 退出循环唯一判据
if ev.IsRunnerCompletion() { break }
```

详见 [Event 字段拆解](../Event字段拆解.md)。

### 3.2 Token Usage 在循环里捕获

```go
// ❌ 错：等 IsRunnerCompletion 才读（Usage 大概率 nil）
if ev.IsRunnerCompletion() { if ev.Response.Usage != nil { ... } }

// ✅ 对：每条都查，取最后一条非零
var lastUsage *model.Usage
for ev := range events {
    if ev.Response != nil && ev.Response.Usage != nil &&
        ev.Response.Usage.TotalTokens > 0 {
        lastUsage = ev.Response.Usage
    }
    ...
}
```

### 3.3 多租户用户 key 模式

```go
// Model 实例不带 APIKey
deepSeekModel = openai.New("deepseek-v4-flash", WithBaseURL(...))

// 每次 Run 自己带
opts := []agent.RunOption{
    agent.WithModelRequestHeaders(map[string]string{
        "Authorization": "Bearer " + userAPIKey,  // 用户自己的 key
    }),
}
```

**核心结论**：同一进程同一组 Model 实例，可以服务任意多用户，每个用户带自己的 key——这是服务端 Agent 和客户端 Agent 的本质区别。

### 3.4 Agent 零默认 prompt（不为错误兜底）

```go
// ❌ 反模式：Agent 层设默认 prompt
chatAgent = llmagent.New("...",
    llmagent.WithGlobalInstruction("你是 EDITH..."),  // ← 让"裸 Run 也能跑"
)

// ✅ 正解：所有 L1~L4 强制从 Run opts 注入
chatAgent = llmagent.New("...",
    llmagent.WithModel(miniMaxModel),
)
```

**Why**：强制显式 + 错误可见 + 责任清晰。漏写 `WithGlobalInstruction` → LLM 裸跑 → 立刻发现。

### 3.5 MCP 平台只能用 HTTP

EDITH 是 Agent 服务平台，**只能支持 HTTP 传输**（`streamable_http` 或 `sse`）：

| 传输 | EDITH 能用？ | 原因 |
|---|---|---|
| STDIO | ❌ | 平台不能下载 server 到机器上 |
| SSE / Streamable HTTP | ✅ | 远端 MCP server，URL 接入 |

### 3.6 ctx 取消跨层传播

完整传播链：

```
Runner.Run(ctx, ...)               ← 用户传入 ctx
  └─> LLM.GenerateContent(ctx, ...)  ← 透传给 Model
       └─> OpenAI SDK HTTP(ctx, ...)   ← 透传给 HTTP
            └─> Tavily MCP HTTP(ctx, ...)  ← 透传给 MCP
                 └─> context deadline exceeded   ← 在这里被取消
```

每一层都正确透传 `ctx`——Go 服务端 Agent 的核心优势。

### 3.7 多租户 MCP 模式（关键！）

**EDITH 是多租户 Agent 平台**——每个用户可能配不同的 MCP server / 不同的鉴权方式。最稳妥的做法：**每个用户每次 Run 重建 ToolSet**。

```go
// 假装 DB（实际从 user_config 表读）
var userMCPConfigs = map[string][]userMCPConfig{
    "alice": {{Name: "tavily", URL: ".../?tavilyApiKey=alice-key"}},
    "bob":   {{Name: "tavily", URL: ".../?tavilyApiKey=bob-key"}},
}

// 加载用户 MCP 工具（返回 tools + 需要 Run 完才能 Close 的 sets）
func loadUserMCPTools(ctx, userID) ([]tool.Tool, []*mcp.ToolSet) { ... }

func run(...) {
    extraTools, mcpSets := loadUserMCPTools(ctx, userID)
    defer func() {
        for _, ts := range mcpSets { ts.Close() }  // ← Close 必须在 run() 返回时 defer
    }()
    edithRunner.Run(ctx, ..., agent.WithAdditionalTools(extraTools))
}
```

**为什么不复用进程级 ToolSet？** MCP server 鉴权不统一（URL param / Header / OAuth），每用户每 Run 重建最简单稳妥，不依赖框架 `WithHTTPBeforeRequest` 那套机制。

**踩坑警告**：❌ 在 `loadUserMCPTools` 里 `defer ts.Close()` —— 函数返回时立刻 Close，Runner.Run 还没开始连接就死了，工具调用报 `transport is closed`。**defer 必须放在 run() 函数体里**。

## 4. 实测数据样本

### 4.1 多用户 × 多模型 × 多性格（独立验证）

| 用例 | userID | model | L1 性格 | L3 主题 | 实测表现 |
|---|---|---|---|---|---|
| 1 | alice | deepseek | 暴躁 | Go 并发 | "下一个问题"、"别磨蹭" |
| 2 | bob | minimax | 温和+比喻 | Python async | "温柔"、"随时告诉我" |

### 4.2 Tavily MCP 真实搜索

用例 2 用 MiniMax M3 调 Tavily，搜索"今日科技新闻"：

- ✅ Tool 名 `tavily_search`（ToolSet 名 + 远端工具名前缀）
- ✅ LLM 主动调 tool，自动填 `query`
- ✅ 真实返回 5 条新闻结果（Anthropic Opus 5、英伟达融资、长鑫科技上市等）
- ✅ Token：prompt=16298 completion=558 total=16856

### 4.3 Token 统计

| 用例 | prompt | completion | total |
|---|---|---|---|
| alice / deepseek / 简单 | 543 | 49 | 592 |
| bob / minimax / Tavily | 16298 | 558 | 16856 |
| alice / deepseek / Tavily | 2574 | 124 | 2698（ctx 取消前） |

## 5. 复现方式

```bash
cd forward
go build .
./forward.exe
```

需要：
- 真实 `deepseek` API key
- 真实 `MiniMax` API key  
- 真实 `Tavily` API key（[申请地址](https://tavily.com/api-keys)）

## 6. EDITH BFF 落地建议

forward/ 验证完的所有模式，**直接照搬**到 EDITH BFF：

| forward/ | EDITH BFF |
|---|---|
| `run(ctx, userID, modelName, apiKey, l1, l3, msg, extraTools)` | `runner.Run(ctx, userID, sessionID, msg, opts...)` |
| 硬编码常量 | 从 DB 读（user_config） |
| 2 个用例 | 多个渠道（telegram / http / 内部 API） |
| Tavily 单 MCP | 每个用户自己的 MCP server list |

**核心不变**：
- Agent 零默认 prompt
- Model 实例不带 key，Run 级 Authorization
- 事件循环正确写法
- 每条事件查 Usage
- ctx 透传到全链路

## 7. 后续可做

1. **接 Telegram Bot** —— forward/ 模式 + Telegram SDK = EDITH BFF
2. **接 HTTP API** —— 同上，换成 HTTP handler
3. **Session 管理** —— 框架自带 SessionService，验证 userID/sessionID 复用
4. **知识库** —— 框架 `WithKnowledge` + `WithKnowledgeFilter`（预留位置）

## 8. 一句话总结

> **EDITH 服务端 Agent 改造方案的可行性已验证**——
> 7 个核心 RunOptions + FunctionTool + MCP + ctx 取消机制，全部跑通。
> 
> 接下来可以直接把 forward/ 模式搬进 BFF。