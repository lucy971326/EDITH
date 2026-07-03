# Webhook Payload 关键字段

> 来源：Issue Comment 事件（action: created）

## MVP 必用字段

| 字段路径 | 示例值 | 用途 |
|---|---|---|
| `action` | `"created"` | 事件动作，我们只处理 created |
| `issue.number` | `1` | Issue 编号，回复和关联 PR |
| `issue.title` | `"测试"` | Issue 标题，给 LLM 上下文 |
| `issue.body` | `"@openswe 你能看到嘛"` | Issue 描述，完整问题内容 |
| `comment.body` | `"你好啊 测试下"` | 评论内容，提取用户指令 |
| `comment.user.login` | `"lucy971326"` | 评论者，防止 App 自己回复自己 |
| `repository.full_name` | `lucy971326/sysdash-tui` | 仓库地址，clone 用 |
| `repository.clone_url` | `https://github.com/lucy971326/sysdash-tui.git` | git clone 地址 |
| `repository.default_branch` | `main` | 默认分支名 |
| `installation.id` | `144161965` | 安装实例 ID，获取 API Token |

## 辅助字段（后续版本用）

| 字段路径 | 用途 |
|---|---|
| `sender.login` | 触发者身份 |
| `issue.labels` | Issue 标签，可做触发条件 |
| `issue.assignees` | 被分配的人 |
| `repository.private` | 是否私有仓库，决定 clone 方式 |
