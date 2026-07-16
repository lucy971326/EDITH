# AG-UI Clients

可运行的 client 前端，用于消费 example servers 暴露的 AG-UI SSE stream。

## Available Clients

- [tdesign-chat/](tdesign-chat/) – 基于 TDesign 构建的 Vite + React chat UI，展示了如何处理 custom events、graph interrupts 以及 report side panels。
- [copilotkit/](copilotkit/) – 基于 CopilotKit 构建的 Next.js web chat，在浏览器中渲染 AG-UI responses。
- [raw/](raw/) – 极简的 Go terminal client，用于打印每个 SSE event 以便进行检查。
- [event_emitter/](event_emitter/) – 展示如何处理 custom events、progress updates 以及从 NodeFunc streaming text 的 Go client。
