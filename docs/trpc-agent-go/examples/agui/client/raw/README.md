# Raw AG-UI SSE Client

这个极简的 terminal client 展示了如何在不需要任何 UI framework 的情况下消费 AG-UI events。它打开一个 SSE stream，使用社区 Go SDK 解析每个 frame，并在事件到达时进行打印，以便你可以逐步观察 agent 的思考过程。

## Run the Client

从 `examples/agui` 路径运行：

```bash
go run .
```

传入 `--endpoint` 可以指定不同的 server URL。Prompts 是通过标准输入（standard input）交互式读取的（Ctrl+D 退出，或输入 `quit`）。

## Sample Output

提交 `calculate 1.2+3.5` 会产生类似以下的输出（为简明起见截断了 ID）：

```text
Simple AG-UI client. Endpoint: http://127.0.0.1:8080/agui
Type your prompt and press Enter (Ctrl+D to exit).
You> calculate 1.2+3.5
Agent> [RUN_STARTED]
Agent> [TEXT_MESSAGE_START]
Agent> [TEXT_MESSAGE_CONTENT] I'll calculate 1.2 + 3.5 for you.
Agent> [TEXT_MESSAGE_END]
Agent> [TOOL_CALL_START] tool call 'calculator' started, id: call_00_rwe3...
Agent> [TOOL_CALL_ARGS] tool args: {"a": 1.2, "b": 3.5, "operation": "add"}
Agent> [TOOL_CALL_END] tool call completed, id: call_00_rwe3...
Agent> [TOOL_CALL_RESULT] tool result: {"result":4.7}
Agent> [TEXT_MESSAGE_START]
Agent> [TEXT_MESSAGE_CONTENT] The result of 1.2 + 3.5 is **4.7**.
Agent> [TEXT_MESSAGE_END]
Agent> [RUN_FINISHED]
```
