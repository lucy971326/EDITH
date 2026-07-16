---
name: dual-verify
description: 派两个子 Agent 独立探索同一问题，主 Agent 对比审核结果，防止单 Agent 幻觉。用于源码分析、架构理解、外部研究、重要事实核查等场景。
---

# 双源验证模式

当任务涉及**源码探索、架构理解、外部研究、重要事实核查**时，必须使用双 Agent 验证。

## 执行步骤

### Step 1: 同时派出两个子 Agent

在同一条消息里发出两个 Agent 调用，让它们**独立探索**：

```
Agent A: 从 X 角度探索 [问题]
Agent B: 从 Y 角度探索 [问题]
```

**关键原则**：
- 两个 Agent **不能互相看到对方的结果**
- 用不同的搜索路径或角度（如 A 读源码，B 读文档）
- 每个 Agent 的 prompt 必须自包含，不引用另一个 Agent 的输出

### Step 2: 等待两个 Agent 都完成

两个 Agent 会并行运行，等待两个都返回结果。

### Step 3: 主 Agent 对比审核

| 情况 | 处理方式 |
|------|---------|
| A 和 B 结论一致 | ✅ 高置信度，直接采信 |
| A 和 B 细节不同 | ⚠️ 找出差异点，深入核实 |
| A 和 B 结论矛盾 | ❌ 不采信任何一方，自己读源码确认 |

### Step 4: 报告结果

输出时标注：
- 哪些结论经过双源验证
- 哪些有差异需要澄清
- 最终可信结论是什么

## 适用场景

- 源码分析（函数调用链、数据流、模块职责）
- 架构理解（组件关系、设计模式）
- 外部研究（技术方案对比、最佳实践）
- 事实核查（API 行为、库的特性）

## 不适用场景

- 简单的文件查找（Glob/Grep 就够了）
- 已知明确答案的问题
- 紧急修复（先修再验证）

## 示例 Prompt

```
# Agent A
Explore the SQLite session service to understand how session_events 
is written. Focus on the addEvent() function in service_helper.go.
Report the exact write condition and which functions call it.

# Agent B  
Investigate the same topic from the runner perspective. How does 
the base runner (not AG-UI) write to the session? Search for 
AppendEvent calls in the runner package.
Report which tables get written and through what code paths.
```
