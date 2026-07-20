# trpc-agent-go 最小 Demo

> 单文件演示：**Runner + LLMAgent + OpenAI 兼容 Model + SQLite Session + Agentic Memory + Mock Tool**

---

## 跑起来

### 1. 准备 LLM

任意 OpenAI 兼容服务都行。下面以 DeepSeek 为例：

```bash
export GITHUB_TOKEN="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"

```


```bash
# bash / Git Bash
export OPENAI_API_KEY="sk-12e81c9adab34fcb9fd9a7a6a699738a"
export OPENAI_BASE_URL="https://api.deepseek.com/v1"
export MODEL_NAME="deepseek-v4-flash" 
```

```bash
export GITHUB_TOKEN="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"
export OPENAI_API_KEY="sk-cp-8Bnw6tM8ZV3JZH74o-c2j-UOoYk3ktO_FDFQGAzNn76Qk4ZLX1NNI492sWI0YwIRqf_NC7kyHel8CrJe7k_hZI2sboFaEp_gAEl8WEgoCHsUbwGJoWC1h8o"
export OPENAI_BASE_URL="https://api.minimaxi.com/v1"
export MODEL_NAME="MiniMax-M3" 
```

```powershell
# PowerShell
$env:OPENAI_API_KEY = "sk-12e81c9adab34fcb9fd9a7a6a699738a"
$env:OPENAI_BASE_URL = "https://api.deepseek.com/v1"
$env:MODEL_NAME = "deepseek-v4-flash"
```
```powershell
# PowerShell
$env:GITHUB_TOKEN="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"
$env:OPENAI_API_KEY = "sk-cp-8Bnw6tM8ZV3JZH74o-c2j-UOoYk3ktO_FDFQGAzNn76Qk4ZLX1NNI492sWI0YwIRqf_NC7kyHel8CrJe7k_hZI2sboFaEp_gAEl8WEgoCHsUbwGJoWC1h8o"
$env:OPENAI_BASE_URL = "https://api.minimaxi.com/v1"
$env:MODEL_NAME = "MiniMax-M3"
```


### 2. 准备 GitHub MCP 凭证

创建只授权测试仓库的 Fine-grained PAT，然后通过环境变量注入：

```bash
export GITHUB_TOKEN="github_pat_xxx"
```

Demo 默认只启用 GitHub `repos`、`issues`、`pull_requests`、`actions`
工具集，并通过 `X-MCP-Readonly: true` 限制为只读操作。

### 3. 跑

```bash
cd Agent专用/trpc-agent-go-demo
go mod tidy
go run .
```

---

## 预期输出

```
━━━━━ Round 1 ━━━━━
👤 Alice: 你好，我叫 Alice，住在杭州，平时喜欢喝美式咖啡。
  🔧 memory_add({"memory": "用户叫 Alice", "topics": ["姓名"]})
  🔧 memory_add({"memory": "用户住在杭州", "topics": ["常住地"]})
  🔧 memory_add({"memory": "用户喜欢喝美式咖啡", "topics": ["偏好"]})
🤖 Assistant: 你好 Alice！很高兴认识你 ☕

━━━━━ Round 2 ━━━━━
👤 Alice: 你还记得我吗？
  🔧 memory_search({"query": "用户个人信息"})
🤖 Assistant: 当然记得啦，你叫 Alice，住在杭州，喜欢美式咖啡对吧？

━━━━━ Round 3 ━━━━━
👤 Alice: 那杭州今天天气怎么样？
  🔧 get_weather({"city": "杭州"})
🤖 Assistant: 杭州今天多云转晴，26°C，适合出门 ☀️

✅ Demo 完成。Session 已持久化到 demo.db，重启后用相同 sessionID 还能继续。
```

**关键观察**：
- Round 1：Agent 主动调 `memory_add` 存三条信息
- Round 2：Agent 调 `memory_search` 找回之前的记忆
- Round 3：Agent 调 `get_weather` 拿天气数据
- 三轮同一个 `userID + sessionID`，对话上下文连续

---

## 整体结构

虽然只有一个 `main.go`，但内部清晰分块：

```go
func main() {
    // 1. Model
    // 2. Session (SQLite)
    // 3. Memory (Agentic)
    // 4. Tools (weather)
    // 5. Agent
    // 6. Runner
    // 7. 多轮对话循环
}

func chat() // 跑一次 Run + 消费事件流

func newWeatherTool() // Mock 天气工具
```

对应框架模块见 [../trpc-agent-go/README.md](../trpc-agent-go/README.md)。

---

## 想试什么改这里

| 想试 | 改哪 |
|---|---|
| 换模型 | `MODEL_NAME` 环境变量，或 `openai.New("xxx")` |
| 换 Session 后端 | 把 `sessionsqlite.NewService(...)` 换成 `inmemory.NewSessionService()` 或 `redis.NewService(...)` |
| 换 Memory 模式 | 把 `NewMemoryService()` 改成 `NewMemoryService(WithExtractor(extractor.NewExtractor(model)))` 进入 Auto 模式 |
| 加 Tool | 仿照 `newWeatherTool()` 写一个，再 append 到 `WithTools(...)` |
| 加多 Agent | `llmagent.WithSubAgents([]agent.Agent{...})` |
| 加插件 | `runner.WithPlugins(plugin.NewLogging(), ...)` |

---

## 关键概念对照

| Demo 代码 | 框架概念 | 对应文档 |
|---|---|---|
| `openai.New(...)` | Model | [../trpc-agent-go/01-核心心智模型.md](../trpc-agent-go/01-核心心智模型.md) |
| `llmagent.New(...)` | Agent | [../trpc-agent-go/03-Agent.md](../trpc-agent-go/03-Agent.md) |
| `runner.NewRunner(...)` | Runner | [../trpc-agent-go/02-Runner.md](../trpc-agent-go/02-Runner.md) |
| `r.Run(ctx, userID, sessionID, msg)` | 异步事件流入口 | [../trpc-agent-go/01-核心心智模型.md#数据流](../trpc-agent-go/01-核心心智模型.md) |
| `ev.IsRunnerCompletion()` | 唯一可靠结束信号 | [../trpc-agent-go/02-Runner.md#5-1-ctx-取消最常用](../trpc-agent-go/02-Runner.md) |
| `sessionsqlite.NewService(...)` | Session 持久化 | [../trpc-agent-go/06-Session与Memory.md](../trpc-agent-go/06-Session与Memory.md) |
| `memoryinmemory.NewMemoryService()` | Memory (Agentic) | [../trpc-agent-go/06-Session与Memory.md#3-memory长期记忆](../trpc-agent-go/06-Session与Memory.md) |
| `function.NewFunctionTool(...)` | Tool | [../trpc-agent-go/04-Tool与MCP.md](../trpc-agent-go/04-Tool与MCP.md) |

---

## 清理

```bash
rm -f demo.db demo.db-shm demo.db-wal
```
