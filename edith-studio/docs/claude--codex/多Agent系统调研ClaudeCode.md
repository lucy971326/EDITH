# 多 Agent 系统调研：Claude Code 自定义 Agent

> 调研方式：Tavily CLI（`tvly search` + `tvly extract`）抓取官方文档与社区资料
> 调研日期：2026-08-08
> 状态：官方文档确认 + 社区佐证

## 结论先行

**Claude Code 完全支持自定义 Agent。** 官方术语为 **Subagent（子代理）**：用一个 markdown 文件（YAML frontmatter + 正文 system prompt）定义一个拥有独立上下文窗口、独立工具集、可选独立模型的专业化助手，主会话通过多种方式把任务委派给它。

> 注意术语区分：**Subagent**（自定义 Agent）与 **Agent 工具**（SDK 内部工具，旧名 `Task`，通过 `subagent_type` 参数拉起 subagent）是两回事。

---

## 1. 如何创建

两种方式：

1. **交互式**：会话内运行 `/agents`，让 Claude 帮你生成。
2. **手写文件**：在 `.claude/agents/`（项目级）或 `~/.claude/agents/`（用户级）放一个 markdown 文件。

官方示例 `~/.claude/agents/code-improver.md`：

```yaml
---
name: code-improver
description: Scans files and suggests improvements for readability, performance, and best practices. Use after writing or modifying code.
tools: Read, Grep, Glob
model: sonnet
---

You are a code improvement specialist. For each issue you find, explain the problem, show the current code, and provide an improved version.
```

### frontmatter 字段表（截止 v2.1.222）

| 字段 | 必填 | 说明 |
|---|---|---|
| `name` | 是 | 唯一标识，**只允许小写字母和连字符**；不允许含 `:`（保留给插件作用域命名 `my-plugin:reviewer`） |
| `description` | 是 | 告诉 Claude 何时应委派给它（自动调用的关键依据） |
| `tools` | 否 | 允许的工具列表；省略则继承 subagent 可用工具全集。**空列表会导致启动失败**（"zero tools"） |
| `disallowedTools` | 否 | 从继承/指定列表中删除的工具；支持 MCP 级模式 `mcp__server__*` |
| `model` | 否 | `sonnet`/`opus`/`haiku`/`fable`/完整模型 ID/`inherit`（默认，跟随主会话） |
| `permissionMode` | 否 | `default`/`acceptEdits`/`auto`/`dontAsk`/`bypassPermissions`/`plan`；**插件 subagent 忽略** |
| `maxTurns` | 否 | 最大智能回合数 |
| `skills` | 否 | 启动时**预加载完整 skill 内容**（不只是 description） |
| `mcpServers` | 否 | 可用 MCP 服务器；**插件 subagent 忽略** |
| `hooks` | 否 | 作用域到该 subagent 的生命周期 hooks；**插件 subagent 忽略** |
| `memory` | 否 | 持久记忆作用域：`user`/`project`/`local` |
| `background` | 否 | `true` 时始终作为后台任务运行 |
| `effort` | 否 | `low`/`medium`/`high`/`xhigh`/`max` |
| `isolation` | 否 | `worktree` 时在临时 git worktree 中运行 |
| `color` | 否 | 任务列表中的显示颜色 |
| `initialPrompt` | 否 | 作为主会话 agent（`--agent` 启动）时的首个用户回合；作为 subagent 调用时忽略 |

正文（frontmatter 之后的所有 markdown）即该 subagent 的 system prompt。它启动时只收到：自己的 system prompt + Agent 工具传入的 prompt、工作目录、项目 CLAUDE.md，**拿不到父会话的历史对话**。

---

## 2. 自定义 Agent vs 自定义 Slash Command

定位完全不同：**slash command 是"注入主上下文的提示词模板"，subagent 是"独立上下文窗口的委派工作者"。**

| 维度 | 自定义 Slash Command | 自定义 Subagent |
|---|---|---|
| 存放位置 | `.claude/commands/<name>.md`（新版推荐迁到 `.claude/skills/<name>/SKILL.md`） | `.claude/agents/<name>.md` 或 `~/.claude/agents/` |
| 运行方式 | 文本**注入主会话同一上下文**（prompt injection），中间输出全占主上下文 | **独立上下文窗口**，只把最终 summary 返回主会话 |
| 系统提示/工具/模型 | 复用主会话 | 可独立配置 |
| 触发方式 | 手动键入 `/name`，或按其 description 自动触发 | 委派、`@` 提及、`claude --agent`、Agent 工具；**没有 `/agent-name` 斜杠触发** |
| 用途 | 可复用长提示词、模板化工作流 | 研究/审查/测试等需要隔离上下文或并行的活 |

> 演进：2026 年更新中 slash command 已与 skills 合并统一——`.claude/commands/` 仍可用，但官方推荐 `.claude/skills/<name>/SKILL.md`；同名时 skill 优先。

---

## 3. 文件放置位置（作用域优先级，数字越小越优先）

| 位置 | 作用域 | 优先级 |
|---|---|---|
| Managed settings | 组织级 | 1（最高） |
| `--agents` CLI 参数 | 当前会话 | 2 |
| `.claude/agents/` | 当前项目 | 3 |
| `~/.claude/agents/` | 所有项目 | 4 |
| 插件的 `agents/` 目录 | 启用该插件的项目 | 5（最低） |

其他细节：
- 支持子目录：`.claude/agents/review/`、`.claude/agents/research/` 等
- 项目级 `.claude/agents/` 建议纳入版本控制供团队共享

---

## 4. 如何触发

**没有 `/agent-name` 这种斜杠触发方式。** 官方确认的触发方式：

1. **自动委派**：description 写精确，Claude 遇到匹配任务自动调用。⚠️ 社区公认**自动选择不稳定**，显式调用才可靠。
2. **自然语言显式要求**：`Use the code-reviewer subagent to look at my recent changes`
3. **`@` 提及**：`@agent-<name>`（如 `@agent-my-plugin:code-reviewer`）
4. **启动参数**：`claude --agent code-reviewer`（作为主会话 agent 运行）
5. **settings 配置**：`.claude/settings.json` 里设 `"agent": "code-reviewer"`
6. **Agent 工具（SDK 场景）**：`query(..., options={agents: {"code-reviewer": AgentDefinition(...)}})` 编程定义，模型通过 Agent 工具按名调用（对应 `subagent_type` 参数）。可用 `tools: Agent(worker, researcher)` 或 `permissions.deny: ["Agent(Explore)"]` 限制/禁用。
7. **`/subtask`**：把当前会话 fork 成继承完整上下文的背景 subagent（与命名 subagent 不同：fork 继承全部对话历史）。

---

## 5. 限制与注意事项

**模型**
- 默认 `inherit`；`CLAUDE_CODE_SUBAGENT_MODEL` 环境变量可全局覆盖默认 subagent 模型。

**工具**
- `tools` 省略则继承全集；空列表导致启动失败。
- 后台 subagent 会过滤部分工具。
- `disallowedTools` 支持 MCP 级删除。
- 预加载 skills 用 `skills` 字段，不要在 `tools` 里列 `Skill`。

**分发**
- **可以通过插件市场分发**：插件根目录 `agents/` 打包，名字带命名空间 `my-plugin:agent-name`。
- 插件 subagent **忽略** `permissionMode`、`mcpServers`、`hooks` 三个字段。

**内置 agent**
- `claude`（继承模型、全工具兜底）、`statusline-setup`（Sonnet）、`claude-code-guide`（Haiku）；另有 `Explore`（Haiku，只读）和 `Plan`，可用 `CLAUDE_CODE_DISABLE_EXPLORE_PLAN_AGENTS=1` 关闭。

**数量/并发**
- `CLAUDE_CODE_MAX_SUBAGENTS_PER_SESSION`、`CLAUDE_CODE_MAX_CONCURRENT_SUBAGENTS`、`CLAUDE_CODE_MAX_SUBAGENT_SPAWN_DEPTH`（默认 1，允许 subagent 再拉 subagent）。

**上下文成本**
- 每次冷启动，有延迟和 token 成本；每个 subagent 的 summary 落回主上下文，大规模 fan-out 会撑大主窗口。
- 不继承父会话系统提示与历史。

**已知坑**
- 自动选择不稳定，显式调用才可靠。
- Opus 有**过度委派** subagent 的倾向（简单任务被委派导致高 token 消耗）——官方 prompt engineering 文档提到。

---

## 6. 对 EDITH Studio 的启示

- Agent 工具 `subagent_type` 参数即对应官方 Agent tool + `agents` map 命名，EDITH 若要复刻该能力，官方路径有两条：
  1. `.claude/agents/*.md` 文件（文件即定义，天然可共享/版本控制）
  2. SDK `agents`/`AgentDefinition` 编程定义（代码内定义，灵活但不可见）
- 关键设计取舍：独立上下文窗口 vs 主窗口注入；`description` 驱动的自动委派 vs 显式调用。

---

## 来源

**官方文档**
- https://code.claude.com/docs/en/sub-agents
- https://code.claude.com/docs/en/agent-sdk/subagents
- https://code.claude.com/docs/en/commands
- https://code.claude.com/docs/en/plugins
- https://code.claude.com/docs/en/agents
- https://code.claude.com/docs/en/agent-sdk/slash-commands

**官方博客**
- https://claude.com/blog/subagents-in-claude-code

**社区/第三方**
- https://www.ksred.com/claude-code-agents-and-subagents-what-they-actually-unlock
- https://jxnl.co/writing/2025/08/29/context-engineering-slash-commands-subagents
- https://joseparreogarcia.substack.com/p/claude-code-agents-explained
- https://news.ycombinator.com/item?id=45181577
- https://www.mindstudio.ai/blog/claude-code-skills-vs-slash-commands

**标注**：frontmatter 字段表、作用域优先级、触发方式、限制等为**官方文档确认**；自动选择不稳定、Opus 过度委派、slash command = prompt injection 等为**社区多源一致报道**（未逐字经官方确认）；内置 agent 清单可能随版本变化，以实际安装版本的文档为准。
