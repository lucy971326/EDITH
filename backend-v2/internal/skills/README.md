# skills

Skills 模块负责加载公共 Skills，并读取当前用户的自定义 Skill 摘要。

```text
main.go
  └─ skills.New({Volumes})
       └─ Module
            └─ Catalog
                 ├─ 启动加载 system Skills
                 ├─ ReadSystemSummaries() → AgentRun
                 └─ ReadUserOverview(userID) → Volume.Service
```

## 文件来源

```text
公共 Skills（代码仓库）
  └─ internal/skills/system/<skill-name>/
       ├─ SKILL.md
       ├─ edith.yaml
       └─ scripts / references / assets

用户 Skills（E2B Volume）
  └─ Volume 根目录
       ├─ overview.md              ← 自动生成的摘要
       └─ <skill-name>/SKILL.md     ← 完整规则
```

`SKILL.md` 的 `name + description` 是摘要来源；`edith.yaml` 只保存页面展示字段。
`overview.md` 是由 `sync_overview.py` 生成的派生索引，不是用户手动维护的真相源。

## 运行链路

```text
AgentRun.Load(userID)
  ├─ Catalog.ListSystemSummaries()
  ├─ Catalog.ReadUserOverview(userID)
  │    └─ Volume.Service.ReadUserOverview(userID)
  │         └─ 读取 Volume 根目录 /overview.md
  ├─ 合并公共摘要 + 用户 overview
  ├─ 一次 agent.WithInstruction(...)
  └─ ManagedRunner.Run
```

这里只读取一个小的 overview 文件，不读取完整 Skill 正文；正文由 Agent 在需要时通过 Sandbox 文件工具读取：

```text
公共：skills/system/<skill-name>/
用户：skills/custom/<skill-name>/
```

## 目录能力

```go
ListSystemSummaries() []SkillSummary
ReadUserOverview(ctx, userID) (string, error)
```

- 公共 Skills 在启动时解析并保存在内存中。
- 用户没有 Volume 或 overview.md 时，用户摘要为空。
- Volume 的 E2B 连接、Token 和路径细节由 `Volume.Service` 隐藏。
- Skills 不创建 Runner、不直接依赖 E2B、不注册 HTTP 路由。

## skill-creator

公共 `skill-creator` 提供三个 Python 标准库脚本：

```text
init_skill.py       创建用户 Skill 骨架
quick_validate.py   校验一个 Skill
sync_overview.py    扫描 custom 并生成 overview.md
```

创建、修改或删除用户 Skill 后，必须重新运行 `sync_overview.py`。Agent 不直接编辑 `overview.md`。
