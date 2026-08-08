# Codex 多 Agent 系统结论

## 结论

Codex 支持多 Agent 和自定义 Agent。

它可以让多个专长不同的 Agent 并行工作，例如：

- `explorer`：只读分析代码
- `reviewer`：审查代码和安全问题
- `worker`：负责实现修改

主 Agent 会收集它们的结果，再统一汇总。

## 自定义 Agent

个人级 Agent：

```text
C:\Users\Administrator\.codex\agents\
```

项目级 Agent：

```text
项目根目录\.codex\agents\
```

每个 Agent 使用一个 `.toml` 文件，例如：

```toml
name = "reviewer"
description = "专门做代码审查"
model = "gpt-5.3-codex"
model_reasoning_effort = "high"
sandbox_mode = "read-only"

developer_instructions = """
只审查代码，不修改文件。
重点检查逻辑错误、安全问题和测试缺失。
"""
```

必须配置三个字段：

```text
name
description
developer_instructions
```

还可以配置专属模型、推理强度、沙箱权限、MCP 和 Skills。

## 和 Skill、AGENTS.md 的区别

| 配置 | 作用 |
|---|---|
| `Skill` | 描述某类任务应该怎么做 |
| 自定义 Agent | 创建一个有独立职责、配置和权限的专家角色 |
| `AGENTS.md` | 规定项目中所有 Agent 都要遵守的通用规则 |

使用时可以直接说：

```text
让 reviewer Agent 审查当前分支，不要修改代码。
```

官方文档：<https://developers.openai.com/codex/subagents>
