# Event 字段拆解

`Event` 是一次 Agent Run 的统一事件信封：模型文本、工具调用、工具结果和最终结束都会经过它。

```text
Event
└─ Response
   ├─ Done / IsPartial
   └─ Choices[0]
      ├─ Message  # 完整消息
      └─ Delta    # 流式增量
```

## 非流式输出

模型一次返回完整内容，主要读取 `Message`：

```text
Event
└─ Response
   ├─ Done: true
   ├─ IsPartial: false
   └─ Choices[0]
      └─ Message
         ├─ Content       # 普通回答，或工具实际返回结果
         ├─ ToolCalls     # 模型请求调用的完整工具
         ├─ ToolName      # 工具结果来自哪个工具
         └─ ToolID        # 对应哪一次工具调用
```

```go
choice := ev.Response.Choices[0]
text := choice.Message.Content
```

## 默认流式输出（当前 OpenAI / DeepSeek Demo）

文本逐块到达，实时展示读 `Delta.Content`；工具调用默认等参数完整后，放在 `Message.ToolCalls`。

```text
第 1、2、3 ... 个文本事件
Event
└─ Response
   ├─ Done: false
   ├─ IsPartial: true
   └─ Choices[0]
      └─ Delta
         ├─ Content          # "深" → "圳" → "今" → "天"
         └─ ReasoningContent # 有思考模型时可能出现

完整工具调用事件
Event
└─ Response
   └─ Choices[0]
      └─ Message.ToolCalls   # get_weather({"location":"深圳"})

工具结果事件
Event
└─ Response
   └─ Choices[0]
      └─ Message
         ├─ ToolName: get_weather
         ├─ ToolID: call_xxx
         └─ Content: {"weather":"雷阵雨"}
```

```go
choice := ev.Response.Choices[0]

// 实时文字
fmt.Print(choice.Delta.Content)

// 完整工具调用
for _, call := range choice.Message.ToolCalls {
	fmt.Println(call.Function.Name, string(call.Function.Arguments))
}

// 工具结果
if choice.Message.ToolID != "" {
	fmt.Println(choice.Message.ToolName, choice.Message.Content)
}
```

## 结束信号

```text
Response.IsFinalResponse()  = 当前模型回复结束
Event.IsRunnerCompletion()  = 整次 Agent Run 真正结束
```

业务代码应以 `ev.IsRunnerCompletion()` 停止读取事件流。

---

## Usage（Token 统计）何时出现

> ⚠️ **踩坑点**：不要等 `IsRunnerCompletion()` 才读 Usage——大概率是 nil。

### 框架逻辑

`model/openai/openai.go:2637-2639` 和 `2785-2788`：

```go
// 只有 token 非零时才挂上 Response.Usage
if usage.PromptTokens > 0 || usage.CompletionTokens > 0 || usage.TotalTokens > 0 {
    finalResponse.Usage = &usage
}
```

### 三种模式的 Usage 出现时机

| 模式 | Usage 在哪条事件上 |
|---|---|
| **非流式** | 唯一那条 Event 必带 Usage |
| **流式** | **累加在每个分片 chunk 里**，最后一条非空 chunk 才完整 |
| **Runner 完成事件** | `runner.completion` 事件**通常不带 Usage**——它是控制信号 |

### EDITH BFF 正确读取方式

```go
// ❌ 错：等 IsRunnerCompletion 才读（大概率 nil）
if ev.IsRunnerCompletion() {
    if ev.Response.Usage != nil { /* token 统计 */ }  // ← 经常进不来
}

// ✅ 对：循环里每条都检查，取最后一条非零
var lastUsage *model.Usage
for ev := range events {
    // ... 处理 Delta / Message ...
    if ev.Response != nil && ev.Response.Usage != nil &&
       ev.Response.Usage.TotalTokens > 0 {
        lastUsage = ev.Response.Usage
    }
    if ev.IsRunnerCompletion() {
        break
    }
}
// lastUsage 就是这次 Run 的最终 token 统计
```

**Why:** 用户在 2026-07-27 forward 验证首次跑通时发现 `IsRunnerCompletion` 触发时 Usage 不打印，根因即此。
