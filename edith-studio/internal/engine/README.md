# engine

Engine 只持有 Workspace 已组装好的长期 Runner、模型目录和 SessionService 能力；它不读取配置文件、不创建工具或 SessionService。

`Run` 使用 Studio 的应用 Context，在同一 goroutine 内消费框架 `frameworkEventCh`，现场读取字段并调用 `send(StreamEvent)`；浏览器断开后继续收尾，不创建应用级第二条事件 channel。

`Compact` 使用本次请求携带的模型选择，并以父 Agent 的 `FilterKey` 同步调用
SessionService 生成摘要；结果由框架保存，Engine 只返回错误。这样未来接入
AgentTool 后，父 Agent 仍按自己的视图读取历史，不会无条件读取整个 Session。

`Run` 的输入支持图片：`RunInput.Images` 携带 `data:image/...;base64,...`，
Engine 校验数量（≤5）、格式（png/jpeg/webp/gif）和单张大小（≤10 MiB），
并要求所选模型 `vision: true`，然后通过 `model.Message.AddImageData` 注入
多模态内容块，由 OpenAI 适配层转成 `image_url` 发送。图片以 base64 内联
保存在 Session 中，是当前版本的已知边界。
