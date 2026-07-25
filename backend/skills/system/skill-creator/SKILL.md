---
name: skill-creator
description: 创建或更新用户私有 Skills。用户要求新增可复用的专业能力、工作流、领域规范，或要求修改已有用户 Skill 时使用。
---

# 创建用户 Skill

将可重复使用的能力保存为用户私有 Skill，使当前及未来会话都能按需使用。

## 创建流程

1. 明确这个能力需要解决的重复任务；不要为一次性任务创建 Skill。
2. 使用简短的小写连字符名称，例如 `thesis-helper`。
3. 创建 `skills/user/<skill-name>/SKILL.md`。
4. 在 `SKILL.md` 顶部写入 `name` 和 `description` 的 YAML 元信息。
5. 正文只保留完成任务必需的流程、约束和工具使用方法；不要写 README、安装说明或变更日志。
6. 需要重复执行的确定性操作时，放入 `scripts/`；较长的规范、资料放入 `references/`，并在 `SKILL.md` 中说明何时读取。
7. 更新 `skills/user/OVERVIEW.md`：每个 Skill 一条短摘要和完整读取路径。
8. 用 `file_read` 分别检查新建的 `SKILL.md` 与 `OVERVIEW.md`。

## SKILL.md 模板

```md
---
name: <skill-name>
description: <一句话说明能力与适用场景>
---

# <技能标题>

<完成这类任务时必须遵守的工作流与约束。保持简洁。>
```

## OVERVIEW.md 格式

```md
- <skill-name>：<一句话能力摘要>
  读取：skills/user/<skill-name>/SKILL.md
```

创建新 Skill 时追加一条；更新已有 Skill 时替换对应条目，不要重复。

## 约束

- 只写入 `skills/user/`，不要修改 `skills/system/`。
- 使用工作目录相对路径，不要使用绝对路径。
- 不要声称存在尚未提供的工具、API 或依赖。
- 优先保持 Skill 简短；模型本身已知的常识不要重复写入。
