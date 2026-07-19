# AG-UI 服务器

本目录展示了可与 AG-UI 客户端示例通信的 AG-UI 服务器。

## 可用服务器

- [`default/`](default/) – 最小化的 AG-UI 服务器，绑定了 `tRPC-Agent-Go` 运行器。
- [`skill_artifacts/`](skill_artifacts/) – 演示 `skill_run` 输出工件如何以 `CustomEvent("tool.artifacts")` 形式暴露。
- [`event_emitter/`](event_emitter/) – 演示如何使用 Node EventEmitter 发送自定义事件、进度更新和 NodeFunc 文本流。
- [`finishresult/`](finishresult/) – 演示通过包装默认翻译器来填充 `RUN_FINISHED.result`。
- [`externaltool/llmagent/`](externaltool/llmagent/) – 演示 `llmagent + agui + WithExternalTools`，在同一对话中包含两个内部工具和两个动态声明的外部工具。
- [`externaltool/graphagent/`](externaltool/graphagent/) – 演示 `GraphAgent` 中断工作流，在同一轮中包含两个内部工具和两个外部工具。
- [`externaltool/agentnode_llmagent/`](externaltool/agentnode_llmagent/) – 演示 AgentNode 子 `LLMAgent` 外部工具，带 AG-UI 中断和恢复。
- [`externaltool/agentnode_graphagent/`](externaltool/agentnode_graphagent/) – 演示包含两个 `AgentNode` 子节点的父 `GraphAgent`，第一个子 `GraphAgent` 内部中断，第二个子 `LLMAgent` 在恢复后运行。
- [`externaltool/agenttool_graphagent_graphagent/`](externaltool/agenttool_graphagent_graphagent/) – 演示父 `GraphAgent` 的 `ToolsNode` 调用 `AgentTool`，其子 `GraphAgent` 通过 AG-UI 中断和恢复。
- [`externaltool/agentnode_handoff_agenttool/`](externaltool/agentnode_handoff_agenttool/) – 演示一个外部 `AgentNode` 产生 `handoff_task` 外部工具调用，由普通图形节点通过动态选择的 `AgentTool` 子 `GraphAgent` 执行。
- [`streamtool/`](streamtool/) – 演示一个最小化的 `StreamableTool`，使用 `agui.WithStreamingToolResultActivityEnabled(true)` 将工具进度作为 `ACTIVITY_SNAPSHOT` / `ACTIVITY_DELTA` 流式传输，同时保留最终的 `TOOL_CALL_RESULT`。
- [`heartbeat/`](heartbeat/) – 演示使用 `agui.WithHeartbeatInterval` 的 SSE 心跳保活帧。
- [`graph/`](graph/) – 演示通过 `ACTIVITY_DELTA` 的图形节点启动活动事件。
- [`react/`](react/) – 服务器展示了 React 规划器标签如何作为自定义 AG-UI 事件流式传输。
- [`langfuse/`](langfuse/) – 本示例展示 AG-UI Server 如何通过 TranslateCallback 自定义报告，并连接到 langfuse 可观测性平台。
- [`report/`](report/) – 以报告为中心的 LLMAgent，将答案作为结构化报告在专用视图中传递，便于消费。
- [`follow/`](follow/) – 启用 `MessagesSnapshot` 跟随模式，使 `/history` 持续流式传输持久化事件直到运行结束。
