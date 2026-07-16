# 消息历史快照示例

本示例展示了如何同时暴露常规的 AG-UI 对话端点与消息历史快照（message snapshot）端点，以便客户端能够按需拉取并回放完整的会话历史记录。

- `server/`：使用 Go 编写的服务端，负责运行 Agent、在内存会话存储（session store）中持久化事件，并启用 `MessagesSnapshot`（消息快照功能）。
- `client/`：一个精简的 TypeScript 客户端脚本，它会首先发起一次对话，随后拉取该会话（thread）对应的历史快照。

有关 AG-UI 中 `MessagesSnapshotEvent` 的具体数据格式，可参考官方文档：[messages](https://docs.ag-ui.com/concepts/messages)。

## 运行服务端

在仓库根目录下运行：

```bash
cd trpc-agent-go/examples/agui/messagessnapshot/server
go run .
```

服务端默认会在 `http://127.0.0.1:8080` 启动，并暴露以下两个端点：

- 对话接口（Chat endpoint）：`http://127.0.0.1:8080/agui`
- 历史快照接口（Snapshot endpoint）：`http://127.0.0.1:8080/history`

你可以使用 `-path` 和 `-messages-snapshot-path` 启动参数来修改这些默认路由路径。

启动输出：

```log
2025-11-05T19:39:53+08:00     INFO     server/main.go:83     AG-UI: serving agent "agui-agent" on http://127.0.0.1:8080/agui
2025-11-05T19:39:53+08:00     INFO     server/main.go:84     AG-UI: messages snapshot available at http://127.0.0.1:8080/history
```

## 运行客户端

打开一个新终端并执行：

```bash
cd trpc-agent-go/examples/agui/messagessnapshot/client
pnpm install
pnpm dev
```

该客户端脚本支持以下环境变量配置：

| 环境变量 | 作用描述 | 默认值 |
|----------|-------------|---------|
| `AG_UI_ENDPOINT` | 对话接口的 URL | `http://127.0.0.1:8080/agui` |
| `AG_UI_HISTORY_ENDPOINT` | 历史快照接口的 URL | `http://127.0.0.1:8080/history` |
| `AG_UI_USER_ID` | 传给服务端的 User 标识符 | `demo-user` |
| `AG_UI_PROMPT` | 用于触发对话的 Prompt 内容 | 示例数学计算问题 |
| `AG_UI_THREAD_ID` | 会话（Thread）ID | 自动生成的时间戳 |

配置运行示例：

```bash
AG_UI_USER_ID=alice pnpm dev
```

客户端的运行逻辑是：首先将 Prompt 发送给对话端点并打印实时的流式响应；接着，使用相同的 `threadId` 和 `userId` 请求历史快照端点，并打印出由服务端返回的完整历史消息日志。

输出结果样例如下：

```log
⚙️ Send chat request to -> http://127.0.0.1:8080/agui
🤖 assistant: I'll help you calculate 2*(10+11) step by step.

First, let's calculate what's inside the parentheses: 10 + 11
🛠️ tool(call_00_6n7vRxRDjtKl0JiAGpHUUw5E): {"result":21}
🤖 assistant: Now we have 2 * 21. Let's calculate that:
🛠️ tool(call_00_z2GLz0y5qf4X0BqRifgN51Pk): {"result":42}
🤖 assistant: **Process explanation:**
1. First, we follow the order of operations (PEMDAS/BODMAS) which tells us to calculate what's inside parentheses first
2. We calculated 10 + 11 = 21
3. Then we multiplied 2 × 21 = 42

**Final conclusion:** 2*(10+11) = **42**
⚙️ Load history -> http://127.0.0.1:8080/history
👤 user(demo-user): Please help me calculate 2*(10+11), explain the process, then calculate, and give the final conclusion.
🤖 assistant: I'll help you calculate 2*(10+11) step by step.

First, let's calculate what's inside the parentheses: 10 + 11
🛠️ tool(call_00_6n7vRxRDjtKl0JiAGpHUUw5E): {"result":21}
🤖 assistant: Now we have 2 * 21. Let's calculate that:
🛠️ tool(call_00_z2GLz0y5qf4X0BqRifgN51Pk): {"result":42}
🤖 assistant: **Process explanation:**
1. First, we follow the order of operations (PEMDAS/BODMAS) which tells us to calculate what's inside parentheses first
2. We calculated 10 + 11 = 21
3. Then we multiplied 2 × 21 = 42

**Final conclusion:** 2*(10+11) = **42**
threadId=thread-1762342798902, userId=demo-user
```
