# EDITH Studio TODO

> 只记录已经达成共识的方向；做前再写极简实现计划。

## 当前优先级：内核骨架

- [x] 按 [架构设计](architecture/架构设计.md) 重构为 `main → studio → engine`。
- [x] `main` 只负责启动应用；Engine 内核自行组装 Model、Tools、SessionService 与 Runner。
- [x] 删除工具审批半成品；不保留空包、空接口或预留 channel。
- [x] 收紧流式主路径：`RunInput → frameworkEventCh → StreamEvent → SSE`。
- [x] Event 翻译不新增 channel、goroutine、缓存或事件桥；所有 channel 名称以 `Ch` 结尾。

## 当前优先级：模型能力层

- [ ] 定义 `models` 的配置结构与校验；模型配置是能力的唯一事实，不维护内置热门模型预设。
- [ ] 分离 Provider 连接信息与 Model 能力信息。
- [ ] 每个可用模型至少声明：`provider`、`name`、`context_window`、`max_output_tokens`、输入模态、`tool_calling`、推理控制能力。
- [ ] 根据配置创建启动期静态模型实例，并通过框架 `WithModels` 预注册。
- [ ] 用框架 Provider / Variant 适配厂商协议；EDITH 不手写 DeepSeek、Qwen、GLM、Kimi、MiniMax 等厂商的思考字段。
- [ ] 模型选择只使用配置中明确声明为可用的模型；Coding Agent 默认模型必须支持工具调用。

## 上下文与会话

- [ ] 按项目隔离会话：`userID = workspace:<规范化 ProjectRoot 的稳定哈希>`。
- [ ] 使用 `SessionService` 的 `ListSessions`、`GetSession`、`DeleteSession` 实现会话侧栏与历史，不直接查询 SQLite。
- [ ] 接入 `WithContextThreshold`：模型提供 Context Window，会话策略提供触发比例、最小 token 阈值、摘要模型与摘要提示词。
- [ ] 开启并校验 Token Tailoring、Context Compaction 与会话摘要的配合，重点保护系统提示、最新用户输入和工具调用配对。
- [ ] 第一版采用单一、保守的全局 Token 估算策略；不伪造“每模型精确 tokenizer”配置。

## 依赖模型能力层的后续功能

- [ ] 前端模型选择器：只展示当前配置已声明的模型与能力。
- [ ] 推理控制 UI：按模型声明显示 thinking 开关或 reasoning effort 选项。
- [ ] 图片附件：仅对支持图片输入的模型显示上传；单独设计附件保存、会话引用、重传与清理，不把 Base64 图片无限写入 SQLite Session。
- [ ] MCP 项目级配置与生命周期。
- [ ] 重新设计工具审批：它是 Run 的暂停 / 恢复能力，不再通过 Callback + 事件桥临时拼接。
