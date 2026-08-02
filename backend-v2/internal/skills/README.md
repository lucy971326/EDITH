# skills

Skills 模块负责加载 EDITH 的内置 Skills，并向 AgentRun 提供稳定的摘要目录。

```text
main.go
  └─ skills.New()
       └─ Module
            └─ Catalog
                 ├─ 启动时读取 system/<skill-name>/SKILL.md
                 ├─ 可选读取同目录 edith.yaml
                 ├─ 校验并保存在内存
                 └─ ListSystemSummaries() → AgentRun
```

## 目录格式

```text
internal/skills/system/current-time/
├─ SKILL.md
└─ edith.yaml

internal/skills/system/skill-creator/
├─ SKILL.md
└─ edith.yaml
```

`SKILL.md` 的 YAML 头部是运行元数据，正文暂时只保存，不注入当前请求：

```yaml
---
name: current-time
description: 需要知道当前时间时，调用 get_current_time 工具。
---
```

`edith.yaml` 只负责 Web 展示元数据；缺失字段回退到 `SKILL.md` 的名称和说明。

Skill 可以携带 `references/`、`scripts/`、`assets/` 等资源；Catalog 不解析这些资源，只负责随 Template 一起复制。当前 `skill-creator` 内置了两个标准库脚本：`init_skill.py` 创建用户 Skill 骨架，`quick_validate.py` 校验 Skill 格式。

Template 创建 Sandbox 后，完整 Skill 文件位于：

```text
/home/user/skills/system/<skill-name>/   公共 Skills
/home/user/skills/custom/<skill-name>/   用户 Skills（未来由 Volume 挂载）

Sandbox 文件工具使用工作区相对路径：

```text
skills/system/<skill-name>/
skills/custom/<skill-name>/
```
```

## 与 AgentRun 的关系

```text
Catalog.ListSystemSummaries()
  └─ [{Name, Description}]
       ↓
AgentRun 组装一次 agent.WithInstruction(...)
       ↓
ManagedRunner.Run
```

Skills 不创建 Runner、不访问用户数据，也不注册 HTTP 路由。新增内置 Skill 只需增加目录文件，不需要改 AgentRun 主流程。
