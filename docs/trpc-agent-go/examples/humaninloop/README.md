# Human-in-the-Loop Agent 示例

本示例演示如何使用 LongRunning Function Tool 实现 **Human-in-the-Loop (HIL)** 模式。Agent 处理员工报销请求：小金额自动审批，大金额需经理审批。

## 概述

Human-in-the-Loop 是 AI Agent 系统中的关键模式，用于在特定决策或验证环节引入人工干预。本示例展示如何：

- **暂停 Agent 执行**，等待人工审批。
- **恢复执行**，在收到人工输入后继续（本示例中通过程序模拟）。
- **处理需要外部验证的长耗时操作。**
- **在审批过程中维护状态。**

## 架构

工作流符合典型的 HIL 模式：

```
用户请求 → Agent 分析 → 决策点
                         ↓
                   金额 < $100？
                     ↙        ↘
               自动审批    请求审批（LongRunning）
                  ↓              ↓
            执行报销       等待人工决策（pending）
                                    ↓
                        审批回调（approved/rejected）
                                    ↓
                             恢复 Agent 执行
                                    ↓
                    通过 → 执行报销    拒绝 → 通知用户
```

本示例中，"审批回调"通过程序模拟实现完整的端到端流程，无需外部服务。生产环境中，此处会由外部审批 UI / 服务触发。

## 核心功能

### LongRunning Function Tool

本示例使用 `LongRunningFunctionTool` 实现审批流程：

```go
function.NewFunctionTool(
    askForApproval,
    function.WithLongRunning(true),
    function.WithName("ask_for_approval"),
    function.WithDescription("Ask for approval for the reimbursement."),
)
```

### 程序化审批（示例模拟）

- Agent 调用 `ask_for_approval` 时，工具返回 pending 状态和 `ticket_id`。
- 示例代码自动模拟经理审批，将审批结果作为更新后的 tool result 发回 Agent。
- 这模拟了真实的外部审批回调，但无需用户手动输入。

## 实现细节

### 1. Agent 配置

报销 Agent 配置如下：

- **Model**：DeepSeek Chat，用于智能决策。
- **Tools**：`reimburse` 和 `ask_for_approval` 两个工具。
- **Instructions**：金额阈值判断的明确规则。

### 2. 工具函数

#### `askForApproval`

- **类型**：LongRunning Function Tool。
- **用途**：金额 ≥ $100 时触发审批流程。
- **返回**：pending 状态 + ticket ID。

```go
func askForApproval(i askForApprovalInput) askForApprovalOutput {
    return askForApprovalOutput{
        Status:   "pending",
        Amount:   i.Amount,
        TicketID: "reimbursement-ticket-001",
    }
}
```

#### `reimburse`

- **类型**：标准 Function Tool。
- **用途**：执行报销。
- **返回**：成功状态。

### 3. 工作流状态

系统处理以下状态：

1. **初始请求**：Agent 收到报销申请。
2. **分析**：Agent 判断是否需要审批。
3. **Pending**：等待经理审批（本示例中程序模拟）。
4. **Approved / Rejected**：应用经理决策。
5. **最终动作**：执行报销或通知用户。

## 使用方法

### 运行示例

```bash
cd examples/humaninloop
# 基础用法（内存 SessionService）
go run .

# 指定模型
go run . -model gpt-4o-mini

# 关闭流式输出
go run . -streaming=false
```

### 交互示例

#### 小金额（自动审批）

```
用户：请报销 $50 餐费
🤖 Assistant：我来处理你的 $50 报销申请...
🔧 工具调用：
   • reimburse (ID: call_001)
✅ 工具结果：{"status": "ok"}
🤖 Assistant：你的 $50 餐费报销已自动审批并处理完成。
```

#### 大金额（需审批，模拟）

```
用户：请报销 $200 会议差旅费
🔧 工具调用：
   • ask_for_approval (ID: call_002)
✅ 工具结果：{"status": "pending", "ticket_id": "reimbursement-ticket-001"}
🤖 Assistant：你的 $200 会议差旅报销需要经理审批...

--- 模拟外部审批 ---
--- 发送更新后的 tool result：{"status": "approved", "approver_feedback": "Approved by manager"} ---
🔧 工具调用：
   • reimburse (ID: call_003)
✅ 工具结果：{"status": "ok"}
🤖 Assistant：你的报销已审批通过并处理完成。
```

### 命令行参数

- `-model`：使用的模型名称（默认："deepseek-v4-flash"）。
- `-streaming`：是否启用流式输出（默认：true）。

## 注意事项

- 生产环境中，审批由外部服务或人工 UI 完成，审批结果发回 Agent。本示例通过程序模拟该行为，以展示完整的端到端流程。

## 参考资料

- [Google ADK Long Running Function Tool](https://google.github.io/adk-docs/tools/function-tools/#2-long-running-function-tool)
- [LangGraph Human-in-the-Loop Concepts](https://langchain-ai.github.io/langgraph/concepts/human_in_the_loop/)
