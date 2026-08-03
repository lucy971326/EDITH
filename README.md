# EDITH

[中文](README.zh-CN.md) · An extensible, multi-user server-side AI Agent platform.

EDITH is not a single chat page. It is a long-running **multi-user server-side Agent platform**: each user receives isolated identity, configuration, sessions, Sandboxes, and Skill Volumes while chatting with models, using tools, building reusable skills, scheduling tasks, and retaining session working files.

For a quick experience, visit the [online demo](https://edith.lucyspace.top/).

> Web is the first user channel. IM channels are intentionally left for a future phase.

## What makes EDITH different

- **A real Agent runtime** — every request reaches one Gateway, is assembled into a run by AgentRun, and is executed by `ManagedRunner`.
- **Multi-user isolation** — Clerk identity scopes requests, sessions, configuration, Sandboxes, Volumes, and scheduled jobs; users never share credentials or runspaces.
- **A persistent workspace** — each session has an E2B Sandbox with files, commands, uploads, generated artifacts, and a read-only file panel.
- **Skills that persist across sessions** — built-in skills are shipped with EDITH; personal skills live in the user's E2B Volume and are discovered through a lightweight overview.
- **Automation, not just chat** — scheduled tasks execute through the same Agent entry point and leave their results in normal conversation history.
- **Long conversations stay usable** — the framework creates rolling summaries when a session approaches 40% of the active model's context window. Original history remains intact.
- **A product-oriented UI** — light/dark themes, compact tool and reasoning cards, session navigation, MCP management, Skills discovery, and a session file workspace.

## Screenshots

![Chat workspace](docs/assets/screenshots/chat-workspace.png)

![Extensions](docs/assets/screenshots/extensions.png)

![Scheduled tasks](docs/assets/screenshots/scheduled-tasks.png)

![Sandbox files](docs/assets/screenshots/sandbox-files.png)

![Settings](docs/assets/screenshots/settings.png)

## Features

### Agent conversations

- Streaming replies, reasoning and tool-call cards
- Request-ID based run status and cancellation
- Session-level concurrency protection
- Multi-provider model configuration and per-chat model selection
- Image input and image history hydration
- Context-window-aware rolling session summaries

### Sandbox files

- A separate E2B Sandbox for each `user_id + session_id`
- Browse the current workspace from the chat page
- Upload source files to `/uploads`
- Let the Agent read, transform, generate, and organize files
- Download completed deliverables from `/artifacts`

### Skills and extensions

- Built-in Skills embedded in the server and mounted in every Sandbox
- Per-user custom Skills stored in a persistent E2B Volume
- `overview.md` provides a stable, cheap summary for Agent context and the Extensions page
- Remote HTTP MCP services can be configured, enabled, and managed in the UI

### Scheduled tasks

- One-time and recurring cron jobs
- Per-user timezone and default model support
- Atomic claiming prevents the same task from running twice
- Every scheduled execution uses the same Gateway and appears in its own conversation session

## Architecture at a glance

EDITH keeps channel handling, execution, and infrastructure separate. The main execution path is deliberately small:

```text
WebAdapter / CronAdapter / future IM Adapter
                    │
                    ▼
                Gateway
          identity + request boundary
                    │
                    ▼
                AgentRun
  model + MCP + skills + images + tools + options
                    │
                    ▼
             ManagedRunner
                    │
                    ▼
       neutral Agent stream events
```

The backend is organized as explicit modules. Each module owns its storage, HTTP boundary when it has one, and its public capability; `main.go` only creates modules and connects their capabilities.

```text
backend-v2/
├─ cmd/server/          composition root and process startup
├─ internal/agentrun/   run-option aggregation and ManagedRunner execution
├─ internal/gateway/    unified Agent request boundary
├─ internal/webadapter/ Web request / SSE adapter
├─ internal/cronjob/    cron storage and scheduler
├─ internal/cronadapter/ scheduled-run adapter
├─ internal/sandbox/    E2B Sandbox lifecycle, files, upload and download HTTP
├─ internal/volume/     persistent per-user E2B Volume
├─ internal/skills/     built-in and custom Skill catalog
├─ internal/tools/      Agent ToolSet registry
├─ internal/userconfig/ user settings, providers, MCP and bindings
├─ internal/conversation/ history projection
└─ internal/agentstream/ framework events → neutral stream events
```

For the detailed Chinese architecture mental model, see [Go架构心智模型.md](backend-v2/Go架构心智模型.md).

## Technology

- Backend: Go, `trpc-agent-go`, SQLite
- Frontend: Next.js, React, TypeScript, Tailwind CSS
- Identity: Clerk
- Agent workspace and persistent Skills: E2B Sandbox + Volume

## Run locally

1. Configure environment variables for Clerk, model providers, and E2B.
2. Start the backend:

   ```powershell
   cd backend-v2
   go run ./cmd/server
   ```

3. Start the web application in another terminal:

   ```powershell
   cd web-v1
   npm install
   npm run dev
   ```

4. Open `http://localhost:3000`.

## Repository guide

| Path            | Purpose                                                 |
| --------------- | ------------------------------------------------------- |
| `backend-v2/`   | Current modular backend                                 |
| `web-v1/`       | Next.js web workspace                                   |
| `e2b-template/` | E2B Sandbox template definition                         |
| `docs/`         | Product, architecture, protocol, and design notes       |
| `reference/`    | Read-only upstream source and documentation references  |
| `backend-v1/`   | Earlier implementation kept for learning and comparison |

## Status

The core multi-user server-side Agent platform is complete: conversations, tools, files, Skills, automation, MCP, durable user configuration, and context compression all work together. Future work can add channel adapters such as Feishu, Telegram, or GitHub App without changing the Agent execution core.
