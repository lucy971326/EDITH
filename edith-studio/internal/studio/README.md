# studio

Studio 创建并持有一个 Workspace，提供本地 HTTP 入口：`POST /api/runs` 将 `RunInput` 交给 `Workspace.Engine` 并把 `StreamEvent` 写成 SSE；`POST /api/runs/{requestID}/cancel` 请求停止；`GET /api/files` 与 `GET /api/files/content` 调用 `Workspace.Project`；`GET/DELETE /api/sessions` 调用 `Workspace.Sessions`。

浏览器直接连接 `http://127.0.0.1:8765`；Studio 只对本机 Web 开发地址放行 CORS，不经过 Next.js 代理。

Studio 不读取框架 Event 字段，也不直接持有模型、工具或 SessionService。
