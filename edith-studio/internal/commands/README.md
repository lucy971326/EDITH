# commands

`commands.Module` 是 Workspace 的产品命令层。

```text
Web
  ├─ GET  /api/commands → List()
  └─ POST /api/commands → Execute()
                         └─ Engine.Compact()
```

- `Definition` 只描述 Web 要展示的命令：名称、说明和输入语法。
- `Input` 保存一次命令请求，以及当前模型选择。
- 首版只有 `/compact`；未知命令在后端拒绝，不进入 Session，也不交给模型。
- 该模块不解析框架 Event，不创建 SSE，不保存命令历史。
