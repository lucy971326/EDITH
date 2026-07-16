# RunWithMessages：仅首次注入历史，后续只发最新消息

本示例演示如何用调用方提供的对话历史驱动 Agent。我们构建一段多轮历史（system + user/assistant 几轮），仅在 session 的第一轮 user turn 时传入（或重置后再次传入）。此后的每一轮，只发送最新的 user 消息；Runner 会把它追加进 session，Agent 从 session events 中读取完整上下文。

## 核心要点

- **Session 自动预热** – Runner 在首次使用（session 为空时）将传入的历史转换为 session events。
- **无缝衔接** – 后续 turns 自动持续追加到 session。
- **Streaming CLI** – 在终端里与 Agent 对话，可按需 reset。

## 前置条件

- Go 1.21 或更高版本
- 有效的 API key（OpenAI 兼容）

环境变量：

- `OPENAI_API_KEY`（必填）
- `OPENAI_BASE_URL`（可选，默认 OpenAI）

## 运行

```bash
cd examples/runwithmessages
export OPENAI_API_KEY="your-api-key"
# 可选：export OPENAI_BASE_URL="https://api.openai.com/v1"（或其他 endpoint）

go run main.go -model deepseek-v4-flash -streaming=true
```

Chat 命令：

- `/reset` — 开启全新的 session，并重新注入默认对话
- `/exit` — 退出 demo

可以试着问：

- "Please add 12.5 and 3" → Agent 应调用 `calculate` 工具。
- "Compute 15 divided by 0" → 工具应返回错误。
- "What is 2 power 10?" → 使用 `calculate` 的 `power` 操作。

## 工作原理

- 准备一段多轮的 `[]model.Message`（system + user/assistant few turns）。
- 首次 user 输入时，调用 `RunWithMessages(...)`，传入 `history + latest user`。
- 此后，调用 `r.Run(...)` 仅传入最新 user 消息；Runner 会追加到 session，content processor 会从 session events 中读取完整上下文。

## 与 `agent.WithMessages` 的关系

- 传入 `agent.WithMessages`（或 `runner.RunWithMessages`）会把提供的 history 在首次使用时持久化到 session。Content processor 不读这个选项，它只转换 session events（当 session 没有 events 时，回退到单个 `invocation.Message`）。

注意事项：

- 当显式提供 `[]model.Message` 时，content processor 优先使用这些 messages，跳过从 session events 或单个 `message` 派生内容，以避免重复。
- `RunWithMessages` 将 `invocation.Message` 设为最新的 user 消息，以兼容那些使用初始 user 输入的 graph/flow Agent。
- Runner 默认仍会把 events 持久化到自身的 session service，但在显式提供 messages 时，这个 session 不参与构造 LLM 请求。

## 与 examples/runner 的对比

- `examples/runner` 演示了用 Runner + 服务端 session state 实现多轮 chat。
- `examples/runwithmessages` 展示了一种 stateless 方式：每次运行你完全掌控完整 prompt —— 适合构建上游系统已维护 session 的 middleware 服务。

## 自定义

- 修改初始 system 消息以引导行为。
- 切换 `-streaming=false` 以一次性返回完整响应。
- 通过 `-model` 替换模型（例如 `gpt-4o-mini`、`deepseek-v4-flash`）。

---

更多细节，参考文档：

- English: `docs/mkdocs/en/runner.md` → "Pass Conversation History (no session dependency)"
- 中文: `docs/mkdocs/zh/runner.md` → "传入对话历史（无需使用 Session）"