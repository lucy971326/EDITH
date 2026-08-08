# EDITH Studio：多 Agent 系统开发计划书（最小版）

> 目标：框架内置 `builtin.NewExplorer` 只读探索子 Agent，父 Agent 自主委派，前端以 Agent Card 展示执行过程。
> 已调研并验证（框架 multiagent/team/taskrun 文档 + 真实 DB 数据 + 前后端双路径 dual-verify）。

## 一、已确认的决策

1. **执行机制**：`agenttool.NewTool`（父 Agent 把子 Agent 当工具同步调用）——语义同 Claude Code Task，事件流透传无需新通道。
2. **子 Agent 用框架内置 `builtin.NewExplorer`**，不自建角色：
   - name=`explorer`、description/instruction 框架现成
   - 默认继承父的工具面（claudecode 全工具 + skills + MCP）、model（用户当前选的）
   - 只读是**软约束**（instruction 明确不修改文件），非权限隔离
3. **判据统一**：子事件 = `ParentMetadata != nil`（框架契约：顶层事件为 nil，子事件必有）。**不用 author 判据**（子调用会合成 author="user" 的事件，会搅混）。
4. **不重复渲染**：`StreamInner` 让子最终文本既出现在子事件、又出现在父 tool.response。**agent 卡只渲染 result（父 tool.response），子事件文本不单独展示**。
5. **不回滚验证结论**：子事件同 session 混合存储（已用 DB 证实）；父上下文隔离靠 filterKey 前缀不匹配自动达成，EDITH 不额外处理。

## 二、数据契约（前后端对齐，无转换层）

### 后端 StreamEvent（engine/types.go）
```go
type StreamEvent struct {
    Type   string `json:"type"`
    Author string `json:"author,omitempty"`  // 新增：空 = 父 Agent；非空 = 子 Agent 名
    // 其余现有字段不变
}
```

### 前端类型（web/src/lib/stream.ts）
```ts
// StreamEvent 增加一行
author?: string;

// AssistantBlock 新增递归 agent 变体
type AssistantBlock =
  | { id; type: "reasoning"; content }
  | { id; type: "text"; content }
  | { id; type: "error"; content }
  | { id; type: "tool"; name; arguments; result; status }
  | { id; type: "agent";
      name: string;           // explorer
      task?: string;          // 委派任务（父 request 参数）
      status: "running" | "completed" | "failed";
      currentTool?: string;   // 当前子工具名（前端跟踪）
      result?: string;        // 最终结论（tool.finished 时填充）
      blocks: AssistantBlock[] }  // 子的过程（思考/子工具）
```

### 历史还原块（session/types.go）
```go
type AssistantBlock struct {
    // 现有字段不变
    Type   string
    Status string
    Blocks []AssistantBlock  // 新增：type="agent" 时子块
}
```

### 事件归组规则（前后端同一判据）
```
父 tool.started(explorer)   → 打开 agent 卡骨架（name=explorer, task=arguments.request, status=running）
author=explorer 的事件      → 递归进该卡 blocks（思考/子工具/当前工具）
父 tool.finished(explorer)  → result=聚合内容, status=completed/failed
```

## 三、改动清单

### 后端（5 项）

| # | 文件 | 改动 |
|---|---|---|
| 1 | `internal/tools/tools.go` | 新增 `NewAgentTool() tool.Tool`：`agenttool.NewTool(builtin.NewExplorer(), WithStreamInner(true))`。无参数无 error（内置角色不需自配工具/模型） |
| 2 | `internal/workspace/workspace.go` | 删内联 explorer + agenttool/builtin import；改为 `tools.NewAgentTool()` 挂 `WithTools`；修 :135 错误注释（filterKey 实际 `explorer-uuid` 非 `edith-studio/explorer-*`） |
| 3 | `internal/engine/types.go` | StreamEvent 加 `Author string` |
| 4 | `internal/engine/run.go` | 事件循环读 `ParentMetadata` 判子事件 → StreamEvent 带 author；**块状态按 author 重置**（否则子文本续用父 block ID）；子工具结果不与子文本重复 |
| 5 | `internal/session/history.go` + `types.go` | 还原：父 tool_calls[explorer] 打开 agent 块 → author 非空事件进嵌套 Blocks → 父 tool.response 填 result |

### 前端（3 项）

| # | 文件 | 改动 |
|---|---|---|
| 6 | `web/src/lib/stream.ts` | StreamEvent 加 author；AssistantBlock 加递归 agent 变体 |
| 7 | `web/src/features/chat/apply-event.ts` | 父 tool.started(explorer) 打开 agent 卡；author 非空事件归入该卡 blocks；agent 工具事件跳过顶层 tool 分支 |
| 8 | `web/src/features/chat/chat-timeline.tsx` | BlockView 加 agent 分支（递归自身），放在兜底 tool 分支之前 |

## 四、明确不做（边界）

- 文件定义体系（`.edith/agents/`）、用户自定义角色、多角色（reviewer/worker）
- 硬只读边界（`builtin.WithToolFilter` 过滤掉 write/edit/bash）——第一版只读靠软约束，验证后再收紧
- 用户手动触发 UI、`@` 提及、分支树可视化
- 并行子 Agent 路由（多个 explorer 并行需要按父 toolCallId 关联）
- token 计算、失败/取消的完整对齐
- taskrun 后台异步（第二形态）

## 五、验证

1. 真实对话触发 explorer 委派，Web 显示 Agent Card（折叠：名 + 任务 + 当前工具；展开：子的思考/子工具；结束：result）
2. DB 检查：子事件仍同 session（无新 session 创建）
3. 刷新/切会话回放：历史还原的 Agent Card 结构与流式一致，无重复文本
4. `go test ./...`、Web 类型检查全绿

## 六、职责边界（最终形态）

```text
tools     造工具：NewAgentTool() 用 builtin.NewExplorer 包装成 tool.Tool（不关心谁用）
workspace 组装：拿工具挂父 Agent（不关心工具内部）
engine    实时翻译：框架事件 → StreamEvent（判子事件 parentMetadata）
session   历史翻译：框架事件 → ChatMessage（同一判据）
前端      消费：按 author 归组 + Agent Card 渲染
```
