# 外部工具 AG-UI 服务端

本目录汇集了有关 AG-UI 的示例，展示了如何在“服务端 Agent 流程”与“客户端（调用侧）”之间协同执行工具。

- [`llmagent/`](llmagent/)：演示了 `LLMAgent` 如何通过动态声明的 `WithExternalTools` 选项，将工具的具体执行交由调用侧（前端/客户端）来完成。
- [`graphagent/`](graphagent/)：演示了 `GraphAgent` 的“中断与恢复”机制。其中包含两个在工作流图内部执行的“内部工具”，以及两个交由调用侧执行的“外部工具”。
- [`agentnode_llmagent/`](agentnode_llmagent/)：演示了在 `AgentNode` 子节点中，`LLMAgent` 使用外部工具的场景。子节点仅接收其自身作用域内的外部工具，而父图节点负责处理检查点（checkpoint）中断，并在收到符合 AG-UI 协议的 `role=tool` 消息后恢复执行。
- [`agentnode_graphagent/`](agentnode_graphagent/)：在父 `GraphAgent` 下配置了两个 `AgentNode` 子节点：其中第一个子 `GraphAgent` 会在内部触发中断，当流程恢复后，第二个子 `LLMAgent` 接续运行。
- [`agenttool_graphagent_graphagent/`](agenttool_graphagent_graphagent/)：父 `GraphAgent` 的工具节点（`ToolsNode`）中包含一个 `AgentTool`。该 `AgentTool` 内部的子 `GraphAgent` 会通过 AG-UI 协议触发中断并等待恢复。
- [`agentnode_handoff_agenttool/`](agentnode_handoff_agenttool/)：演示了外层 `AgentNode` 发起一个任务交接（`handoff_task`）的外部工具调用。随后，普通图节点通过动态选择一个被 `AgentTool` 包装的子 `GraphAgent` 来执行该交接任务。
