# session

`session.Module` 创建并持有 `%USERPROFILE%/.edith/sessions.db` 的框架 SQLite SessionService。Workspace 负责关闭 Module；Engine 通过 `Service()` 保存对话和执行压缩。

它也是框架 Session 数据到 Studio 产品数据的唯一边界：

```text
Service()                   Engine 保存对话
List(workspaceID)           左侧会话摘要
Get(workspaceID, sessionID) 历史 ChatMessage
Delete(...)                 删除一个已存在会话
CreateSessionSummary(...)   将会话压缩结果保存到 session_summaries
```

`workspaceID` 显式传入并作为框架 `UserID`；不同项目目录的历史不会互相可见。Web 和 Studio 不直接查询 SQLite 或读取框架 Event。

`Get` 还原历史时会把用户消息中的图片内容块转换为 `ChatMessage.Images`
（data URL），前端无需再次请求文件。

创建时接入框架 `DynamicSummarizer`：每次 Run 或 `/compact` 都从请求 context 读取当前模型选择，再生成摘要器；SessionService 本身只创建一次。
