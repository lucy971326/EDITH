# EDITH

> [中文](README.zh-CN.md) | **English**
>
> A multi-tenant, server-side Agent platform (MVP): web login, streaming chat, session isolation, sandboxed workspaces, and bring-your-own Telegram Bot integration.

## The Name

EDITH is named after the AI system from *Spider-Man: Far From Home*:

> **Even Dead, I'm The Hero.**

What EDITH borrows isn't the film's lore — it's the core idea: **AI that doesn't just chat — it understands tasks, calls tools, and completes real work inside a controlled workspace.**

## What It Does

- **Clerk-authenticated access.** Browser-supplied `user_id` is never trusted — the Next.js BFF injects it from the Clerk session.
- **Streaming chat on the web.** Model selection, reasoning traces, tool calls, and history replay.
- **Vision input.** Images go straight to vision-capable models as visual input.
- **File uploads land in the sandbox.** Regular files upload into the current session's sandbox, where the Agent can read and process them.
- **Workspace browser.** Inspect files the Agent produced and download them.
- **Two sandbox backends.** Local filesystem or E2B cloud sandbox, switched by env var.
- **System Skills.** Public, read-only Skills ship preinstalled in the E2B template; the Agent reads the full instruction on demand before executing.
- **Bring-your-own Telegram Bot.** Users paste their own Bot Token on the page; the backend registers a Webhook and routes messages into that user's Agent session.
- **GitHub MCP toolset** wired in out of the box.

![EDITH main UI](asset/主界面.png)

## Architecture

```text
Web Browser
    │ Clerk login
    ▼
Next.js BFF
    │ injects trusted user_id from the session
    ▼
Go Backend
    │ Gateway → Runner.Run(APPName, user_id, session_id)
    ▼
Agent + Tools + Local / E2B Sandbox
```

```text
User's own Telegram Bot
    │ Webhook: /webhook/telegram/{routeKey}
    ▼
TelegramService
    │ routeKey → ownerUserID + Telegram Client
    ▼
Gateway → Runner → Agent
    │
    └── replies via the same Bot client
```

The isolation boundary:

```text
APPName + user_id + session_id
```

## Repository Layout

```text
.
├── backend/      Go backend: Agent, Gateway, Sandbox, Telegram, HTTP API
├── web/          Next.js: Clerk, chat UI, BFF, SSE
├── docs/         EDITH architecture & learning notes
├── .claude/      project-level Agent / Skill config
└── .codex/      project-level Agent / Skill config
```

## Local Setup

### 1. Configure environment

Copy the env templates in each directory:

```powershell
Copy-Item backend/.env.example backend/.env
Copy-Item web/.env.example web/.env
```

Then fill in:

- `backend/.env` — DeepSeek, MiniMax, GitHub Token; configure E2B and the Telegram Webhook URL if you need them.
- `web/.env` — Clerk publishable key and secret key.

Local Sandbox works out of the box:

```dotenv
SANDBOX_MODE=local
```

### 2. Start the backend

```powershell
cd backend
go run .
```

Default address: `http://127.0.0.1:8080`

### 3. Start the frontend

Open a new terminal:

```powershell
cd web
npm install
npm run dev
```

Then visit: `http://localhost:3000`

## Telegram Integration

1. Create a Bot via BotFather in Telegram and grab the Bot Token.
2. Expose the backend over public HTTPS (e.g. via ngrok).
3. Configure `backend/.env`:

```dotenv
TELEGRAM_WEBHOOK_BASE_URL=https://your-public-domain
```

4. Log in to EDITH, click **Telegram** in the top-right, and paste your Bot Token.

The backend validates the token, generates an internal `routeKey`, and registers the Webhook with Telegram. The frontend never sees the `routeKey`.

If your machine can't reach Telegram's API directly, configure a proxy. **The proxy port must match your Clash (or similar) port**:

```dotenv
TELEGRAM_PROXY=http://127.0.0.1:7897
```

![Model picker and Telegram configuration](asset/模型与Tele配置.png)

## Development Checks

```powershell
# Requires GNU Make. On Windows, install via Git Bash, MSYS2, or Scoop.
make check
make build
```

Common commands:

```text
make backend-run    start the Go backend
make web-dev        start the Next.js development server
make check          run backend tests + frontend TypeScript checks
make build          build backend and web
```

## Known Limitations

- **Telegram Bot configuration is in-memory only.** You'll need to re-enter the Token after every backend restart.
- **Telegram sender verification is not implemented yet.** Anyone who messages your bot lands in your Agent session — see the security note in `docs/IM接入设计.md` before exposing this publicly.
- **E2B requires your own API key.** Local development defaults to Local Sandbox.
- **Secrets, runtime workspaces, and build artifacts must not be committed.** Keep `.env`, keys, and build outputs out of Git.

## Documentation

- [IM integration design](docs/IM接入设计.md) *(中文)*
- [Multi-tenant migration plan](docs/多用户改造计划.md) *(中文)*
- [tRPC-Agent-Go learning notes](docs/learn/trpc-agent-go/01-核心心智模型.md) *(中文)*

## License

[MIT](LICENSE)