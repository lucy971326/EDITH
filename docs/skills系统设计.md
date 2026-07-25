# EDITH Skills 系统设计

## 目标

让 EDITH 像人一样：平时只知道“有哪些专业能力”，需要时才打开对应 Skill 的完整说明。

```text
系统 Skills：EDITH 自带，所有用户可用，不可修改
用户 Skills：用户私有，可安装，也可由 EDITH 直接生成
```

## 核心心智模型

```text
Template = EDITH 的系统盘：环境 + 系统 Skills
Volume   = 一个用户的永久技能盘：用户私有 Skills
Sandbox  = 一个会话的工作电脑：user_id + session_id
```

```mermaid
flowchart LR
  T[EDITH Template\n系统 Skills] --> S1[用户 A 会话 1 Sandbox]
  T --> S2[用户 A 会话 2 Sandbox]
  V[用户 A Skill Volume\n私有 Skills] --> S1
  V --> S2
```

比例关系：

```text
一个 user_id → 一个 Skill Volume → 多个 session Sandbox
```

## Sandbox 内目录

默认工作目录固定为 `/home/user`。

```text
/home/user/
├── uploads/                 当前会话上传文件
├── output/                  当前会话产物
└── skills/                  Agent 唯一需要记住的 Skill 入口
    ├── system/              Template 内置，只读
    └── user/                用户 Volume 挂载，可读写
```

注意：Volume 挂载到 `/home/user/skills/user`，不能覆盖整个 `skills/`，否则看不见 Template 中的 `system/`。

## 系统 Skills

系统 Skill 只维护一份源码：

```text
backend/skills/system/       ← Git 管理，唯一真相
        ↓ 构建 Template 时自动复制
/home/user/skills/system/    ← E2B 内的只读执行副本
```

```text
修改系统 Skill
→ 提交 EDITH 代码
→ 重建 EDITH Template
→ 新建 Sandbox 使用新版
```

暂停或已创建的旧 Sandbox 不会自动获得新 Template；需要重建才会升级。

## 用户 Skills

用户 Volume 根目录下，一个文件夹就是一个 Skill：

```text
用户 Volume
├── thesis-helper/
│   ├── SKILL.md
│   └── scripts/
└── data-cleaner/
    ├── SKILL.md
    └── references/
```

每个 Skill 的 `SKILL.md` 顶部保存最小元信息：

```md
---
name: thesis-helper
description: 按用户指定的论文规范进行润色与检查
---
```

完整正文、脚本、参考资料只在 Agent 真正需要该 Skill 时读取。

## 自动触发：摘要与完整内容分离

每轮 Agent 对话只注入短摘要，而不注入全部 Skill 正文：

```text
固定 EDITH 规则
+ 当前用户可用 Skills 摘要
↓
LLM 判断是否匹配
↓
匹配时读取对应 SKILL.md
```

```mermaid
flowchart LR
  U[user_id] --> SS[SkillService]
  SS --> C[Skill 摘要]
  C --> A[AgentService]
  M[用户消息] --> A
  A --> R[Runner.Run]
  R --> L[LLM]
```

摘要示例：

```text
可用 Skills：
- system/pdf：处理、提取、生成 PDF
- user/thesis-helper：按用户论文格式润色
```

Prompt 分工：

```text
GlobalInstruction = 全体用户相同的 EDITH 核心规则
Instruction       = 当前 user_id 的 Skill 摘要
用户消息           = 用户这次真正说的话
```

`Runner.Run` 仍只负责运行。外层在每次运行前，根据 `user_id` 组装摘要并以 `agent.WithInstruction(...)` 传入；它不是把摘要伪装成用户消息。

## 摘要加载与缓存

```text
系统摘要
→ 启动时直接扫描 backend/skills/system/
→ 缓存一次

用户摘要
→ 缓存命中：直接返回
→ 缓存未命中：E2B Volume SDK 列目录、读取各 SKILL.md 顶部描述
```

```mermaid
flowchart LR
  U[user_id] --> K{用户摘要缓存}
  K -->|命中| R[返回摘要]
  K -->|未命中| V[E2B Volume]
  V --> M[列出 Skill 文件夹]
  M --> P[读取 SKILL.md 描述]
  P --> R
```

缓存只是优化，不是正确性的前提。用户或 Agent 直接改了 Volume 后，前端可提供“重新加载 Skills”按钮：清掉该 `user_id` 的摘要缓存，下次运行重新读取。

## Sandbox 生命周期

```mermaid
flowchart TD
  A[收到 user_id + session_id] --> B{已有 sandbox_id？}
  B -->|有| C[连接或恢复旧 Sandbox]
  C --> D[保留创建时的 Volume 挂载]
  B -->|没有| E[确保该用户 Volume 存在]
  E --> F[创建 Sandbox\nEDITH Template + Volume 挂载]
  F --> G[保存 sandbox_id]
```

规则：

```text
新建 Sandbox：挂载用户 Volume
Pause / Resume：沿用原挂载，不能中途新增挂载
旧 Sandbox 未挂 Volume：销毁后按新版配置重建
```

MVP 可通过稳定名称避免新增用户-Volume 映射表：

```text
volumeName = "edith-skills-" + hash(user_id)
```

## 用户创建与安装 Skills

### EDITH 直接生成

用户要求新能力时，EDITH 可以直接写入：

```text
/home/user/skills/user/<skill-name>/
```

该目录已挂载用户 Volume，因此写入即永久保存。Agent 不需要用户再次确认“发布”。

### 外部安装

第一版优先提供“从 GitHub 安装 Skill”，而不是先做 zip 上传：

```text
用户输入 GitHub Skill URL
→ POST /skills/install
→ SkillService 下载对应 Skill 文件夹
→ 写入用户私有 Skill 库
→ 前端重新加载 Skills
```

GitHub 安装、未来 zip 上传、未来技能市场，最终都落到同一处：

```text
用户私有 Skill Volume
```

## 职责边界

```text
Runner / Agent 核心：运行推理，不认识 Template、Volume、安装流程
AgentService：按 user_id 获取摘要，并调用 Runner.Run
SkillService：管理摘要、系统 Skill 来源、用户 Skill 安装
E2B Provider：创建/恢复 Sandbox，负责 Template 与 Volume 挂载
Sandbox：执行 Skill 脚本和读写当前工作目录
```

## 暂不处理

```text
- 外部 GitHub URL / 压缩包安全校验
- Skill 版本、更新、依赖管理
- 多会话同时修改同一个用户 Skill 的冲突策略
- Telegram 的 reload Skills 命令
```

## 已验证的 E2B 行为

后端通过 Volume SDK 写入文件后，已挂载该 Volume 的运行中 Sandbox 可以立即读取到新文件。

```text
实测结果：约 397ms 可见
```

因此 GitHub 安装流程可以直接由后端 SDK 写入用户 Volume；当前会话 Sandbox 无需重建，即可使用新安装的 Skill。

可复现实验：`backend/test/e2b_volume_visibility`。
