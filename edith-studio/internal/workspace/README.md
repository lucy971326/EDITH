# workspace

`workspace.Workspace` 是一个 `ProjectRoot` 对应的长期产品运行对象。它创建并持有项目、会话、模型、工具和 Agent 内核；Studio 直接使用它公开的真实能力字段。

```text
Studio
  ↓ Create(ProjectRoot)
Workspace
  ├─ Project     文件树与文本读取
  ├─ Sessions    SQLite SessionService
  ├─ Models      已注册模型实例
  ├─ toolSets    内部工具资源
  └─ Engine      Agent Run 与取消
```

`Create` 按依赖顺序创建资源；中途失败会关闭已经创建的后续资源。`Close` 按反向顺序关闭 Engine、ToolSets 和 Sessions，并合并全部关闭错误。
