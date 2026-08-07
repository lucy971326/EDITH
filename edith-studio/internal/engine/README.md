# engine

Engine 只持有 Workspace 已组装好的长期 Runner、模型目录和 SessionService 能力；它不读取配置文件、不创建工具或 SessionService。

`Run` 使用 Studio 的应用 Context，在同一 goroutine 内消费框架 `frameworkEventCh`，现场读取字段并调用 `send(StreamEvent)`；浏览器断开后继续收尾，不创建应用级第二条事件 channel。

`Compact` 使用本次请求携带的模型选择，并以父 Agent 的 `FilterKey` 同步调用
SessionService 生成摘要；结果由框架保存，Engine 只返回错误。这样未来接入
AgentTool 后，父 Agent 仍按自己的视图读取历史，不会无条件读取整个 Session。
