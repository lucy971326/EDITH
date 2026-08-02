---
name: skill-creator
description: 创建或更新 EDITH Skill，把可复用的工作方法、领域知识和资源组织成结构清晰的 Skill。当用户要求新增 Skill、修改已有 Skill、为某类任务沉淀流程时使用。
---

# EDITH Skill 创建指南

Skill 用来描述 Agent **如何完成一类任务**；Tool 用来执行具体动作。

```text
Skill = 工作方法、规则、判断流程
Tool  = 文件操作、进程操作、外部 API 等执行能力
```

## 存放位置

EDITH 有两类 Skill：

```text
skills/system/<skill-name>/   公共 Skill：平台提供，只读
skills/custom/<skill-name>/   用户 Skill：当前用户自己的内容，可编辑
skills/custom/overview.md      用户 Skills 摘要索引，由脚本生成
```

创建或修改用户 Skill 时，只能使用 `skills/custom/` 下的路径，不能修改 `skills/system/`。
所有路径都必须是 Sandbox 工作区相对路径，不能使用绝对路径或 `..`。

## 目录结构

最小 Skill 只需要一个 `SKILL.md`：

```text
<skill-name>/
├─ SKILL.md          必须：摘要和完整规则
├─ edith.yaml        可选：EDITH 页面展示信息
├─ references/       可选：按需读取的详细资料
├─ scripts/          可选：需要稳定执行的脚本
└─ assets/           可选：模板、图片等输出资源
```

`SKILL.md` 必须以 YAML 头部开始，并且只保留 `name` 和 `description`：

```markdown
---
name: daily-summary
description: 帮助用户创建每日总结任务；当用户提到日报、每日总结或定时提醒时使用。
---

# 每日总结

这里写 Agent 执行这类任务时必须遵守的规则和步骤。
```

`name` 使用小写字母、数字和连字符，目录名与 Skill 名保持一致；`description` 同时说明能力和触发场景。

`edith.yaml` 只负责页面展示，不参与 Agent 的触发判断：

```yaml
display_name: 每日总结
short_description: 创建和管理每日总结任务。
```

## 编写原则

1. **只写模型不知道的内容**：删除常识和空泛背景，优先保留规则、边界和决策条件。
2. **先写稳定流程**：把必须遵守的步骤写清楚；可变化的部分给 Agent 合理选择空间。
3. **保持短小**：`SKILL.md` 只放主流程，详细资料放到 `references/`，脚本放到 `scripts/`。
4. **渐进加载**：先让 Agent 看到摘要，只有任务需要时才读取正文和资源。
5. **一层引用**：`SKILL.md` 直接链接参考文件，不要继续嵌套多层引用。
6. **不要堆无关文件**：不要创建 README、安装说明、变更日志等辅助文档。

## 创建流程

按下面顺序完成，不要只写一个空目录：

```text
确定 Skill 的任务边界和触发场景
        ↓
确定 name、description 和目录名
        ↓
创建 skills/custom/<skill-name>/
        ↓
写入 SKILL.md 的摘要和执行规则
        ↓
按需添加 references、scripts 或 assets
        ↓
重新读取并检查格式、路径和规则是否完整
```

使用 Sandbox 文件工具创建用户 Skill：

1. 用 `sandbox_make_directory` 创建目录。
2. 用 `sandbox_write_file` 写入 `SKILL.md` 和 `edith.yaml`。
3. 用 `sandbox_read_file` 重新读取文件，检查 YAML 头部、正文和展示字段。
4. 用 `sandbox_list_files` 确认目录只包含实际需要的文件。
5. 运行 `sync_overview.py`，更新 `skills/custom/overview.md`。

Skill 自带三个可复用脚本：

```text
skills/system/skill-creator/scripts/init_skill.py
skills/system/skill-creator/scripts/quick_validate.py
skills/system/skill-creator/scripts/sync_overview.py
```

需要创建骨架时运行 `init_skill.py`；完成创建、修改或删除后运行 `quick_validate.py` 和 `sync_overview.py`。
`overview.md` 是自动生成的摘要索引，不能手动编辑。这些脚本只使用 Python 标准库，不依赖额外安装包。

## 更新已有 Skill

- 先读取现有 `SKILL.md`，理解原有触发条件和规则。
- 只修改用户自己的 `skills/custom/` 内容。
- 保留稳定的 `name`；除非用户明确要求，不要随意改名。
- 修改后重新读取完整文件，确认摘要仍准确、正文没有互相矛盾的规则。
- 修改后重新运行 `sync_overview.py`，保持用户 Skills 摘要索引同步。

## 运行时边界

```text
Skills 摘要 ──► AgentRun ──► ManagedRunner
完整正文 ────► Agent 按需调用 Sandbox 文件工具读取
```

不要把完整 Skill 正文无条件复制到每次请求的指令中；不要把 Skill 当成 Tool，也不要在 Skill 中伪造用户身份或越过 Sandbox 的路径限制。
