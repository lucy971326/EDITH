# EventEmitter Client

该 client 连接到 EventEmitter server 示例，并展示富文本格式的 custom events、progress updates 以及流式 text events。

## Prerequisites

首先，启动 server：

```bash
cd examples/agui
go run ./server/event_emitter
```

## Running the Client

在新的终端中：

```bash
cd examples/agui
go run ./client/event_emitter
```

### Options

| Flag | Default | Description |
|------|---------|-------------|
| `-endpoint` | `http://127.0.0.1:8080/agui` | AG-UI SSE endpoint |
| `-prompt` | `process my data` | 要发送的 user prompt |

### 使用 Custom Prompt 的示例

```bash
go run ./client/event_emitter -prompt "analyze this dataset"
```

## Expected Output

```
╔══════════════════════════════════════════════════════════════╗
║       EventEmitter Client - Node Custom Events Demo          ║
╚══════════════════════════════════════════════════════════════╝

📡 Connecting to: http://127.0.0.1:8080/agui
📝 Sending prompt: "process my data"

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
                         Event Stream
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

🚀 [run_started] Run started
   Thread: event-emitter-demo-thread, Run: run-1234567890

🎬 [workflow.started] Workflow initiated
   ⏰ Timestamp: 2024-01-01T12:00:00Z
   📥 User input: "process my data"
   📌 Version: 1.0.0

📊 [process] ██████░░░░░░░░░░░░░░░░░░░░░░░░  20.0% - Processing step 1 of 5
📊 [process] ████████████░░░░░░░░░░░░░░░░░░  40.0% - Processing step 2 of 5
📊 [process] ██████████████████░░░░░░░░░░░░  60.0% - Processing step 3 of 5
📊 [process] ████████████████████████░░░░░░  80.0% - Processing step 4 of 5
📊 [process] ██████████████████████████████ 100.0% - Processing step 5 of 5

📝 [analyze] 📊 Starting analysis...
📝 [analyze] 📝 Input received: "process my data"
📝 [analyze] 🔍 Analyzing patterns...
📝 [analyze] ✅ Pattern analysis complete.
📝 [analyze] 📈 Generating insights...
📝 [analyze] 💡 Key findings:
📝 [analyze]    - Data processed successfully
📝 [analyze]    - No anomalies detected
📝 [analyze]    - Performance metrics within expected range

🎉 [workflow.completed] Workflow finished
   ⏰ Timestamp: 2024-01-01T12:00:03Z
   📤 Result: Analysis completed successfully with no issues found.
   ✅ Status: Success
   ⏱️  Duration: 2500ms
   🔗 Nodes: start → process → analyze → complete

🏁 [run_finished] Run completed
   Thread: event-emitter-demo-thread, Run: run-1234567890

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

✅ Demo completed successfully!
```

## Event Types Displayed

| Event Type | Icon | Description |
|------------|------|-------------|
| `workflow.started` | 🎬 | workflow 开始时的 custom event |
| `workflow.completed` | 🎉 | workflow 结束时的 custom event |
| `node.progress` | 📊 | 展示操作状态的 progress bar |
| `node.text` | 📝 | 流式 text 输出 |
| `run_started` | 🚀 | AG-UI run lifecycle event |
| `run_finished` | 🏁 | AG-UI run lifecycle event |
| `custom` | ⚡ | 通用 custom events |

## Understanding the Demo

1. **Start Node** (`workflow.started`): 发送一个带有 workflow metadata 的 custom event
2. **Process Node** (`node.progress`): 通过 progress bar 展示实时的 progress updates
3. **Analyze Node** (`node.text`): 逐行流式传输 text 输出
4. **Complete Node** (`workflow.completed`): 发送带有 summary 的最终结果
