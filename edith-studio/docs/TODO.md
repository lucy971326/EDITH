# EDITH Studio TODO

> 只记录已经达成共识的方向；开始实现前先写极简计划。

## 已完成

- [x] 按架构文档组织 `studio → workspace → engine`。
- [x] `Workspace` 管理一个启动目录及其产品能力；`Engine` 只负责 Agent Run。
- [x] 保持唯一流式主路径：`Runner → frameworkEventCh → Engine 现场处理 → StreamEvent → SSE`。
- [x] 删除工具审批半成品；不保留空包、空接口、额外事件 channel 或事件桥。
- [x] 按启动目录生成 `WorkspaceID`，不同项目目录的 Session 互相隔离。
- [x] 使用框架 `SessionService` 保存、查询和删除会话。
- [x] 项目文件树按目录展开读取，文件按点击读取；路径限制在 `ProjectRoot` 内。
- [x] `models.Module` 负责读取 `~/.edith/models.yaml`，并在启动时创建全部模型实例。
- [x] 模型配置分离 Provider 连接信息与 Model 能力信息。
- [x] 模型声明 `context_window`、`vision`、`thinking` 能力。
- [x] 使用框架 Provider / Variant 处理厂商协议差异。
- [x] 后端通过 `/api/models` 暴露不含密钥的模型目录。
- [x] Web 根据后端模型目录选择模型和思考模式；单次 Run 可以切换已注册模型。
- [x] 完成 Go 测试、竞态测试、静态检查和 Web 类型检查。

### 会话上下文管理

- [x] 使用框架 Context Threshold / Context Compaction 与摘要 API；不自行实现 tokenizer。
- [x] 将模型 `ContextWindow` 交给框架，按 80% 比例触发自动压缩。
- [x] 自动滚动摘要 + DynamicSummarizer（摘要模型 = 当前 Run 的模型选择）。
- [x] 工具结果上下文压缩，不破坏系统提示、用户输入、工具调用与结果的配对。
- [x] 摘要是派生数据：生成失败只返回错误，不删除原始 Session Event。
- [x] 后端 `/api/sessions/{id}/context` 提供上下文用量事实，Web 只展示。
- [x] Slash Command 与手动 `/compact`：命令目录、后端二次校验、`CreateSessionSummary(force=true)`。

### 多模态图片输入

- [x] 仅 `vision: true` 模型可发图，前后端双重校验（≤5 张、png/jpeg/webp/gif、单张 ≤10 MiB）。
- [x] `AddImageData` 注入内容块 → OpenAI 适配层转 `image_url`，历史读取还原为 data URL。

### Web 交互

- [x] 会话新建、切换、历史加载、删除的完整交互。
- [x] 模型能力驱动的选择器：思考模式、上下文窗口、图片按钮。
- [x] 编辑器文件标签、只读 Monaco、工具/思考/错误卡片统一展示。

### MCP 扩展

- [x] 用户级与项目级 `mcp.json` 配置，项目级覆盖同名 server。
- [x] 支持 stdio / sse / streamable_http 三种传输；stdio 密钥直接写入配置 `env`。
- [x] 单 server 连接失败不阻塞启动，状态经 `GET /api/mcp` 展示在侧栏扩展区。
- [x] MCP ToolSet 汇入 Agent，模型侧工具名为 `{server}_{tool}`。
- [x] 项目级配置目录 `.edith/` 已 gitignore，密钥不入库。

### Skills

- [x] 用框架知识注入层（`WithSkills` + `SkillToolProfileKnowledgeOnly`），执行层不用（技能脚本走既有 bash 工具）。
- [x] 系统/用户/项目三级 skills 目录发现；运行时同名覆盖（项目 > 用户 > 系统），展示层累积。
- [x] `GET /api/skills` 返回三级列表；侧栏 Skills 弹层（上下键 + ENTER 技能名进输入框，LLM 自主 `skill_load`）。
- [x] LoadMode session（跨轮保留）+ 正文物化到 tool result（不用 system）。详细决策见 `docs/product/Skills系统设计指南.md`。
- [x] 系统级内置技能：`//go:embed` 进二进制，每次启动 `SeedSystemSkills` 全量物化到 `~/.edith/.system-skills/`（含 skill-creator，已适配 EDITH 路径）。
- [x] 内置 skill-creator：删 openai.yaml 生成逻辑、`quick_validate` 去 PyYAML 依赖、脚本强制 LF 行尾。

### 本机工具层（方案 c）

- [x] claudecode 本地化到 `internal/claudecode`：`normalizePath` 放开 base_dir（文件工具全盘访问，可读写 `~/.edith` 技能目录），bash 平台分派（Windows 指 Git Bash，避开 WSL 启动器）。
- [x] 启动自动创建用户/项目级技能目录；Glob 不存在目录文案不误导；测试隔离修复。详细计划见 `docs/product/工具集改造计划.md`。

## 当前优先级

1. 文件附件、文件编辑、Diff、Agent 修改后的刷新策略。
2. 工具权限：重新设计为 Run 的暂停 / 恢复能力，不再用 Callback + 事件桥拼接。
3. 子 Agent、分支、运行状态和更多 Web 产品能力。

## 暂不做

- 图片存储外置（ContentRef）：不需要。本地单用户的 SQLite 撑得住图片；后期前端做图片压缩即可。
- 多用户权限与审计。
- 项目级模型配置覆盖。
- 会话历史导出与云同步。
- 生产环境打包、自动更新和远程部署。
