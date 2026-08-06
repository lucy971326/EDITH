# session

`session.Module` 创建并持有 `%USERPROFILE%/.edith/sessions.db` 的框架 SQLite SessionService。Workspace 负责关闭 Module；Engine 只通过 `Service()` 使用它保存对话。

它也是框架 Session 数据到 Studio 产品数据的唯一边界：

```text
Service()                   Engine 保存对话
List(workspaceID)           左侧会话摘要
Get(workspaceID, sessionID) 历史 ChatMessage
Delete(...)                 删除一个已存在会话
```

`workspaceID` 显式传入并作为框架 `UserID`；不同项目目录的历史不会互相可见。Web 和 Studio 不直接查询 SQLite 或读取框架 Event。
