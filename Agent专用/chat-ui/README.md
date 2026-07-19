# chat-ui

Go trpc-agent-go AG-UI Server 的最简前端（CopilotKit + Next.js）。

## 跑法

两个终端分别起：

**终端 1：Go 后端（Agent + AG-UI Server）**
```bash
cd ../trpc-agent-go-demo
export OPENAI_API_KEY=sk-xxx
export OPENAI_BASE_URL=https://api.deepseek.com/v1
export MODEL_NAME=deepseek-v4-flash
export GITHUB_TOKEN=github_pat_xxx
go run .
```

后端会在 `http://127.0.0.1:8080/agui` 起一个 AG-UI Server。

**终端 2：前端**
```bash
pnpm install
cp .env.local.example .env.local
pnpm dev
```

打开 http://localhost:3000 即可对话。

## 修改后端地址

改 `.env.local`：
```
AG_UI_ENDPOINT=http://your-server:8080/agui
```

## 架构

```
浏览器 (:3000)
   ↓ POST /api/copilotkit
Next.js route.ts (CopilotRuntime + HttpAgent)
   ↓ HTTP POST (AG-UI 协议 JSON)
Go HTTP Server (:8080/agui)
   ↓ SSE 流
trpc-agent-go runner
   ↓
LLM + Tools + Memory + Session
```
