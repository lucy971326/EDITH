# engine

Engine 在启动时组装模型、Claude Code ToolSet、SQLite SessionService 与一个长期 Runner。

`Run` 使用 Studio 的应用 Context，在同一 goroutine 内消费框架 `frameworkEventCh`，现场读取字段并调用 `send(StreamEvent)`；浏览器断开后继续收尾，不创建应用级第二条事件 channel。
