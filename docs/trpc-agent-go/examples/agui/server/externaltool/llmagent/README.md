# LLMAgent 外部工具 AG-UI 服务端

本示例展示了如何暴露一个由 `LLMAgent` 支持的 AG-UI SSE 端点。该 Agent 同时混合使用了以下两类工具：

- **内部工具**（如 `calculator`、`internal_lookup`）：由框架在后台自动执行。
- **外部工具**（如 `external_note`、`external_approval`）：在 AG-UI 请求中声明，并由客户端（调用侧）通过 `role=tool` 消息在外部执行并返回结果。

本示例旨在验证 `agui + llmagent + agent.WithExternalTools(...)` 方案在单次会话中处理“混合工具流”的能力，具体交互流程如下：

1. **第一次调用（`role=user`）**：声明了外部工具 `external_note` 和 `external_approval`。随后，Agent 会输出全部四个工具的调用事件（tool calls）。
2. **框架自动执行**：框架会立即执行内部工具 `calculator` 与 `internal_lookup`，并返回它们的 `TOOL_CALL_RESULT` 事件。
3. **等待外部输入**：对于外部工具 `external_note` 和 `external_approval`，框架会暂缓处理，结束本次运行（run），并等待调用侧返回结果。
4. **第二次调用（`role=tool`）**：调用侧将这两个外部工具的执行结果，作为尾随的 `role=tool` 消息发送回服务端。
5. **给出最终解答**：大模型从历史记录中读取全部四个工具的执行结果，并生成最终回答。

在进行第二次的 `role=tool` 后续请求时，会复用相同的 `threadId`，但会生成一个新的 `runId`。

默认的 AG-UI 运行器（runner）会自动转换 `input.Tools` 并将其传递给 `agent.WithExternalTools(...)`。因此，这些外部工具在 Go 后端不需要具体的 `Call` 方法实现。

## 运行

在 `examples/agui` 模块下执行以下命令：

```bash
export OPENAI_API_KEY="your-api-key"
export OPENAI_BASE_URL="https://your-openai-compatible-base-url" # 可选
go run ./server/externaltool/llmagent \
  -model gpt-4.1-mini \
  -address 127.0.0.1:8080 \
  -path /agui
```

此外，服务端还在 `http://127.0.0.1:8080/history` 启用了消息历史快照（`MessagesSnapshot`）接口。

## 验证

验证脚本 `run.sh` 会自动发送两个 `curl` 请求：

- 第一个 `curl` 请求发送 `role=user` 消息。
- 脚本从该请求的 SSE 响应中提取出 `external_note` 和 `external_approval` 的 `toolCallId`。
- 第二个 `curl` 请求发送 `role=tool` 消息，并带上刚才提取出的这两个 `toolCallId` 及对应的执行结果。

在仓库根目录下运行：

```bash
bash ./examples/agui/server/externaltool/llmagent/run.sh
```

如果你想查看持久化保存的历史记录，可以在这两步流程完成后，单独请求 `/history` 接口。
