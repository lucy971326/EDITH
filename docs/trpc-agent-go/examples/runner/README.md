# Runner 快速入门：带工具的多轮对话

本示例演示了一个最简多轮对话界面，使用 `Runner` 编排组件。它以内置内存会话后端为核心功能，易于理解和运行。

## 什么是多轮对话？

本实现展示了构建对话式 AI 应用所需的核心特性：

- **🔄 多轮对话**：在多次交互中保持上下文
- **🌊 灵活的输出**：支持流式（实时）和非流式（批量）响应模式
- **💾 会话管理**：对话状态的持久化与连续性
- **🔧 工具集成**：可正常执行的计算器和时间工具
- **🚀 简洁的界面**：干净、专注的聊天体验

### 核心特性

- **上下文保持**：助手记住之前的对话轮次
- **灵活响应模式**：可选择流式（实时）或非流式（批量）输出
- **会话连续性**：聊天会话期间一致的对话状态
- **工具调用执行**：正确执行和展示工具调用过程
- **工具可视化**：清晰展示工具调用、参数和响应
- **错误处理**：优雅的错误恢复与报告

## 前置条件

- Go 1.21 或更高版本
- 有效的 OpenAI API 密钥（或兼容的 API 端点）

## 环境变量

| 环境变量 | 描述 | 默认值 |
| --- | --- | --- |
| `OPENAI_API_KEY` | OpenAI 模型的 API 密钥 | `` |
| `OPENAI_BASE_URL` | OpenAI 模型 API 端点的基础 URL | `https://api.openai.com/v1` |
| `ANTHROPIC_AUTH_TOKEN` | Anthropic 模型的 API 密钥 | `` |
| `ANTHROPIC_BASE_URL` | Anthropic 模型 API 端点的基础 URL | `https://api.anthropic.com` |

## 命令行参数

| 参数 | 描述 | 默认值 |
| --- | --- | --- |
| `-model` | 要使用的模型名称 | `deepseek-v4-flash` |
| `-variant` | 调用 OpenAI 提供商时使用的变体 | `openai` |
| `-streaming` | 启用流式响应模式 | `true` |
| `-enable-parallel` | 启用并行工具执行（更快的性能） | `false` |

## 使用方法

### 基本对话

```bash
cd examples/runner
export OPENAI_API_KEY="your-api-key-here"
go run .
```

### 自定义模型

```bash
export OPENAI_API_KEY="your-api-key"
go run . -model gpt-4o
```

### 自定义变体

```bash
export OPENAI_API_KEY="your-api-key"
go run . -variant deepseek
```

### 响应模式

可选择流式和非流式响应：

```bash
# 默认流式模式（实时字符输出）
go run .

# 非流式模式（一次性返回完整响应）
go run . -streaming=false
```

**各模式适用场景：**

- **流式模式**（`-streaming=true`，默认）：适用于交互式聊天，希望实时看到响应，提供即时反馈和更好的用户体验。
- **非流式模式**（`-streaming=false`）：适用于自动化脚本、批量处理，或需要先获取完整响应再进行后续处理的场景。

### 工具执行模式

控制当 AI 进行多次工具调用时多个工具的执行方式：

```bash
# 默认串行工具执行（安全且兼容）
go run .

# 并行工具执行（更快的性能）
go run . -enable-parallel=true
```

**各模式适用场景：**

- **串行执行**（默认，无需额外标志）：
  - 🔄 工具逐个按顺序执行
  - 🛡️ **安全且兼容**的默认行为
  - 🐛 更适合调试工具执行问题
- **并行执行**（`-enable-parallel=true`）：
  - ⚡ 多次工具调用时**更快的性能**
  - ✅ 最适合独立工具（计算器 + 时间，天气 + 人口）
  - ✅ 使用 goroutine 同时执行工具

### 帮助与可用选项

查看所有可用的命令行选项：

```bash
go run . --help
```

输出：

```
Usage of ./runner:
  -enable-parallel
        启用并行工具执行（默认：false，串行执行）
  -model string
        要使用的模型名称（默认 "deepseek-v4-flash"）
  -variant string
        调用 OpenAI 提供商时使用的变体名称（默认 "openai"）
  -streaming
        启用响应流式模式（默认 true）
```

## 已实现的工具

本示例包含两个可用的工具：

### 🧮 计算器工具

- **函数名**：`calculator`
- **操作**：加、减、乘、除
- **用法**："Calculate 15 \* 25" 或 "What's 100 divided by 7?"
- **参数**：operation（字符串），a（数字），b（数字）

### 🕐 时间工具

- **函数名**：`current_time`
- **时区**：UTC、EST、PST、CST 或本地时间
- **用法**："What time is it in EST?" 或 "Current time please"
- **参数**：timezone（可选字符串）

## 工具调用过程

当你请求计算或时间信息时，你会看到：

```
🔧 工具调用已发起：
   • calculator (ID: call_abc123)
     Args: {"operation":"multiply","a":25,"b":4}

🔄 执行工具中...
✅ 工具响应 (ID: call_abc123): {"operation":"multiply","a":25,"b":4,"result":100}

🤖 助手：我为你计算了 25 × 4 = 100。
```

## 对话界面

界面简洁直观：

```
🚀 Runner 快速入门：带工具的多轮对话
Model: deepseek-v4-flash
Streaming: true
Parallel tools: false
Session backend: in-memory (simple demo)
Type '/exit' to end the conversation
Available tools: calculator, current_time
==================================================
✅ Chat ready! Session: demo-session-1703123456

👤 You: Hello! How are you today?
🤖 Assistant: Hello! I'm doing well, thank you for asking. I'm here and ready to help you with whatever you need. How are you doing today?

👤 You: I'm good! Can you remember what I just asked you?
🤖 Assistant: Yes, I can! You just asked me how I was doing today, and I responded that I'm doing well. This shows that I'm maintaining context from our conversation. Is there anything specific you'd like to chat about or any way I can help you?

👤 You: /exit
👋 Goodbye!
```

## 停止与取消

- 要干净地结束聊天循环，输入 `/exit` 并按下回车。
- 要随时停止整个程序，按下 **Ctrl+C**。
- 要在你自己的应用中安全停止**单个**运行的 agent 执行，取消你传递给 `Runner.Run` 的 `context.Context`（不要停止读取事件通道）。参见：
  - `examples/cancelrun`（通过 Enter/Ctrl+C 取消，排空事件）
  - `examples/managedrunner`（通过 requestID 取消，分离取消）
  - `docs/mkdocs/en/runner.md`（运行控制指南）

## 会话存储

本示例使用**内存会话存储**以保持简洁。这意味着：

- ✅ 快速且无外部依赖
- ✅ 非常适合开发与测试
- ⚠️ 程序退出时会话数据会丢失

**生产环境使用需要持久化会话存储**（Redis、PostgreSQL、MySQL），请参见 `examples/session/` 目录，该目录演示了高级会话管理功能，包括：

- 多种会话后端（Redis、PostgreSQL、MySQL）
- 使用 `/use <id>` 命令切换会话
- 使用 `/sessions` 命令列出会话
- 使用 `/new` 命令创建新会话
