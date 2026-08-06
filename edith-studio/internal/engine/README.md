# engine

Engine 只持有 Workspace 已组装好的长期 Runner 和当前项目的 Session 隔离身份；它不读取模型配置、不创建工具或 SessionService。

`Run` 使用 Studio 的应用 Context，在同一 goroutine 内消费框架 `frameworkEventCh`，现场读取字段并调用 `send(StreamEvent)`；浏览器断开后继续收尾，不创建应用级第二条事件 channel。
