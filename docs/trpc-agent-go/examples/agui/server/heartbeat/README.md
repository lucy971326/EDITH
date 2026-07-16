# 心跳机制 AG-UI 服务端

本示例展示了如何在 AG-UI 中使用 SSE 心跳保活（heartbeat keepalive）帧。

本示例将 `agui.WithHeartbeatInterval` 配置集成进了一个常规的 AG-UI 服务端中。该服务端底层接入了真实的 `LLMAgent`（使用兼容 OpenAI 的大模型）以及一个真实的 `FunctionTool`。我们引导 Agent 在回答前调用 `wait_before_answer` 工具。在此工具执行等待期间，服务端通过持续写入心跳注释（comment）帧，来保持 SSE 连接的活跃状态。

## 示例展示的重点

- 在默认的 SSE 传输层上配置 `agui.WithHeartbeatInterval(d)`。
- 当 Agent 运行（run）处于活跃状态且暂无 AG-UI 事件可发送时，发送 SSE 注释帧（`:\n\n`）进行保活。
- 工具等待期（静默期）前后的常规 AG-UI 事件流。
- 通过 `function.NewFunctionTool` 实现由大模型驱动的真实工具调用。

## 运行

在 `examples/agui` 模块下执行以下命令：

```bash
export OPENAI_API_KEY="your-api-key"
export OPENAI_BASE_URL="https://your-openai-compatible-base-url"

go run ./server/heartbeat \
  -model deepseek-v3.2 \
  -address 127.0.0.1:8080 \
  -path /agui \
  -heartbeat 1s \
  -wait 5s
```

服务端会暴露以下接口：

- 对话端点：`http://127.0.0.1:8080/agui`

## 体验一下

可以通过 `curl` 来观察原始的 SSE 流：

```bash
curl -N http://127.0.0.1:8080/agui \
  -H 'Content-Type: application/json' \
  -d '{
    "threadId": "heartbeat-demo",
    "runId": "heartbeat-run-1",
    "messages": [
      {
        "role": "user",
        "content": "Wait before answering, then say hello."
      }
    ]
  }'
```

在原始输出中，当 Agent 处于运行状态时，心跳帧会夹在正常的 `data:` 帧之间显示。一个包含工具等待期的事件流样例如下：

```text
data: {"type":"RUN_STARTED",...}
...
data: {"type":"TOOL_CALL_START",...}
data: {"type":"TOOL_CALL_ARGS",...}
data: {"type":"TOOL_CALL_END",...}
:

:

data: {"type":"TOOL_CALL_RESULT",...}
...
data: {"type":"TEXT_MESSAGE_START",...}
data: {"type":"TEXT_MESSAGE_CONTENT",...}
data: {"type":"TEXT_MESSAGE_END",...}
data: {"type":"RUN_FINISHED",...}
```

其中以单冒号 `:` 开头的行就是来自 SSE 传输层的心跳帧。

## 参数说明

- `-heartbeat`：心跳发送间隔。设为 `0` 则禁用心跳帧。
- `-wait`：`wait_before_answer` 工具在执行过程中的等待时长（即静默期）。
