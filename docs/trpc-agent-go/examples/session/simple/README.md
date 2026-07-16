# Session Management Demo

本示例演示了如何使用 `Runner` 组件实现高级 session 管理能力，展示了如何在多个对话 session 之间切换，并支持多种不同的存储后端。

## 什么是 Session Management？

该实现展示了 session 管理在对话式 AI 中的强大能力：

- **Multiple Sessions（多 Session）**：创建并在多个独立的对话上下文之间切换
- **Storage Options（存储选项）**：支持 no-op、in-memory、SQLite、Redis、PostgreSQL、pgvector、MySQL 以及 ClickHouse 后端
- **Session Discovery（Session 发现）**：列出已有 session 并自由切换

### 关键特性

- **Session Creation**：使用 `/new` 创建新的对话 session
- **Session Switching**：使用 `/use <id>` 切换 session
- **Session Listing**：使用 `/sessions` 查看所有活跃 session
- **History Recap**：使用 `/history` 让 agent 总结对话内容
- **Semantic Recall**：后端实现了 `session.SearchableService` 时，可使用 `/search <query>` 进行语义检索
- **Backend Flexibility**：可从 no-op、in-memory、SQLite、Redis、PostgreSQL、pgvector、MySQL 或 ClickHouse 存储中选择
- **Context Preservation**：每个 session 维护独立的对话历史
- **Langfuse Tracing**：通过 Langfuse 为 Redis session 操作提供可选的 OpenTelemetry 链路追踪

## 前置条件

- Go 1.21 或更高版本
- 有效的 OpenAI API key（或兼容的 API endpoint）
- 可选：SQLite 文件、Redis 服务器、PostgreSQL 服务器、带 `pgvector` 的 PostgreSQL、MySQL 服务器或 ClickHouse 服务器（取决于后端选择）

## 环境变量

**必须设置：**

| 变量                     | 描述                              | 必填      | 默认值                        |
| ------------------------ | --------------------------------- | --------- | ----------------------------- |
| `OPENAI_API_KEY`         | OpenAI 模型的 API key             | **是**    | -                             |
| `OPENAI_BASE_URL`        | OpenAI 模型 API endpoint 的 Base URL | **是**  | `https://api.openai.com/v1`   |

### 后端特定变量

**SQLite：**
| 变量                  | 描述       | 默认值                                   |
| --------------------- | ---------- | ---------------------------------------- |
| `SQLITE_SESSION_DSN`  | SQLite DSN | `file:sessions.db?_busy_timeout=5000`    |

**Redis：**
| 变量           | 描述              | 默认值            |
| -------------- | ----------------- | ----------------- |
| `REDIS_ADDR`   | Redis 服务器地址  | `localhost:6379`  |

**PostgreSQL：**
| 变量           | 描述                | 默认值              |
| -------------- | ------------------- | ------------------- |
| `PG_HOST`      | PostgreSQL 主机     | `localhost`         |
| `PG_PORT`      | PostgreSQL 端口     | `5432`              |
| `PG_USER`      | PostgreSQL 用户     | `root`              |
| `PG_PASSWORD`  | PostgreSQL 密码     | ``                  |
| `PG_DATABASE`  | PostgreSQL 数据库   | `trpc-agent-go`     |

**PGVector：**
| 变量                        | 描述             | 默认值                            |
| --------------------------- | ---------------- | --------------------------------- |
| `PGVECTOR_HOST`             | PostgreSQL 主机  | `localhost`                       |
| `PGVECTOR_PORT`             | PostgreSQL 端口  | `5432`                            |
| `PGVECTOR_USER`             | PostgreSQL 用户  | `postgres`                        |
| `PGVECTOR_PASSWORD`         | PostgreSQL 密码  | ``                                |
| `PGVECTOR_DATABASE`         | PostgreSQL 数据库| `trpc-agent-go-pgsession`         |
| `PGVECTOR_EMBEDDER_MODEL`   | Embedding 模型   | `text-embedding-3-small`          |

可选的专用 embedding 凭证：

| 变量                             | 描述                 | 默认值                               |
| -------------------------------- | -------------------- | ------------------------------------ |
| `OPENAI_EMBEDDING_API_KEY`       | Embedding API key    | 回退到 `OPENAI_API_KEY`              |
| `OPENAI_EMBEDDING_BASE_URL`      | Embedding API Base URL | 回退到 `OPENAI_BASE_URL`          |

**MySQL：**
| 变量            | 描述        | 默认值              |
| --------------- | ----------- | ------------------- |
| `MYSQL_HOST`    | MySQL 主机  | `localhost`         |
| `MYSQL_PORT`    | MySQL 端口  | `3306`              |
| `MYSQL_USER`    | MySQL 用户  | `root`              |
| `MYSQL_PASSWORD`| MySQL 密码  | ``                  |
| `MYSQL_DATABASE`| MySQL 数据库| `trpc_agent_go`     |

**Langfuse Tracing（可选）：**

| 变量                   | 描述                                    | 默认值              |
| ---------------------- | --------------------------------------- | ------------------- |
| `LANGFUSE_SECRET_KEY`  | Langfuse secret key                     | -                   |
| `LANGFUSE_PUBLIC_KEY`  | Langfuse public key                     | -                   |
| `LANGFUSE_HOST`        | Langfuse 主机（host:port，不含 scheme） | -                   |
| `LANGFUSE_INSECURE`    | 使用 HTTP 而非 HTTPS（`true`/`false`）  | `false`             |

## 命令行参数

| 参数               | 描述                                            | 默认值             |
| ------------------ | ------------------------------------------------ | ------------------ |
| `-model`           | 要使用的模型名称                                  | `MODEL_NAME` 环境变量 |
| `-session`         | Session 后端：noop/inmemory/sqlite/redis/postgres/pgvector/mysql/clickhouse | `redis` |
| `-streaming`       | 启用流式响应模式                                   | `true`             |
| `-event-limit`     | 每个 session 最大存储的事件数                      | `1000`             |
| `-session-ttl`     | Session 的存活时间（time-to-live）                 | `10s`              |
| `-search-topk`     | `/search` 最多返回的召回事件数                     | `5`                |
| `-debug`           | 启用调试模式，打印 session 事件                    | `true`             |
| `-enable-trace`    | 启用 Langfuse 追踪 session 操作                    | `false`            |

## 使用方法

### 使用 In-Memory 后端

```bash
cd examples/session/simple
export OPENAI_API_KEY="your-api-key-here"
export OPENAI_BASE_URL="https://api.openai.com/v1"
go run . -session inmemory
```

### 使用 No-Op 后端

当需要 runner/session 集成但不希望在对话轮次之间持久化历史记录时，可使用 no-op 后端：

```bash
export OPENAI_API_KEY="your-api-key-here"
export OPENAI_BASE_URL="https://api.openai.com/v1"
go run . -session noop
```

### 自定义 Event Limit 和 Session TTL

控制存储的事件数量以及 session 的存活时间：

```bash
# 每个 session 最多存储 200 个事件，TTL 为 48 小时
go run . -event-limit 200 -session-ttl 48h

# 存储 50 个事件，TTL 为 6 小时（适用于测试）
go run . -event-limit 50 -session-ttl 6h
```

**Event Limit**：控制内存使用量和查询性能。数值越小 = 内存占用越少，查询越快。

**Session TTL**：非活跃 session 在被清理前保留的时间。TTL 越长 = 对回访用户的体验越好。

### 使用 Redis 后端

```bash
export OPENAI_API_KEY="your-api-key"
export OPENAI_BASE_URL="https://api.openai.com/v1"
go run . -session redis
```

自定义 Redis 地址：
```bash
export OPENAI_API_KEY="your-api-key"
export OPENAI_BASE_URL="https://api.openai.com/v1"
export REDIS_ADDR="localhost:6380"
go run . -session redis
```

### 使用 PostgreSQL 后端

```bash
export OPENAI_API_KEY="your-api-key"
export OPENAI_BASE_URL="https://api.openai.com/v1"
export PG_PASSWORD="your-password"
go run . -session postgres
```

自定义 PostgreSQL 配置：
```bash
export OPENAI_API_KEY="your-api-key"
export OPENAI_BASE_URL="https://api.openai.com/v1"
export PG_HOST="localhost"
export PG_USER="postgres"
export PG_PASSWORD="your-password"
export PG_DATABASE="sessions_db"
go run . -session postgres
```

### 使用 PGVector 后端

```bash
export OPENAI_API_KEY="your-api-key"
export OPENAI_BASE_URL="https://api.openai.com/v1"
export PGVECTOR_HOST="localhost"
export PGVECTOR_USER="postgres"
export PGVECTOR_PASSWORD="your-password"
export PGVECTOR_DATABASE="trpc-agent-go-pgsession"
export PGVECTOR_EMBEDDER_MODEL="text-embedding-3-small"
export OPENAI_EMBEDDING_API_KEY="$OPENAI_API_KEY"
export OPENAI_EMBEDDING_BASE_URL="$OPENAI_BASE_URL"
go run . -session pgvector
```

pgvector 后端激活后，聊天循环还会暴露语义召回功能：

```text
You: /search travel plan
Semantic recall for "travel plan":
   1. [0.927] assistant ...
```

### 使用 MySQL 后端

```bash
export OPENAI_API_KEY="your-api-key"
export OPENAI_BASE_URL="https://api.openai.com/v1"
export MYSQL_PASSWORD="your-password"
go run . -session mysql
```

自定义 MySQL 配置：
```bash
export OPENAI_API_KEY="your-api-key"
export OPENAI_BASE_URL="https://api.openai.com/v1"
export MYSQL_HOST="localhost"
export MYSQL_USER="root"
export MYSQL_PASSWORD="your-password"
export MYSQL_DATABASE="sessions_db"
go run . -session mysql
```

### 使用 SQLite 后端

SQLite 是本地持久化的不错默认选择，无需运行外部数据库服务器。

```bash
export OPENAI_API_KEY="your-api-key"
export OPENAI_BASE_URL="https://api.openai.com/v1"
export SQLITE_SESSION_DSN="file:sessions.db?_busy_timeout=5000"
go run . -session sqlite
```

### 使用 Langfuse Tracing

使用 Redis 后端时，可以启用 Langfuse tracing 来在 Langfuse 控制台中观察 session 操作（如 create_session、get_session、append_event 等）。

该示例在每次 `runner.Run()` 调用前创建一个 root span，使得所有 session spans 通过 context 传播成为该 root span 的子 span。这样做是必要的，因为 session 操作由 Runner 在 Agent 的 `Run()` 调用*之前和之后*执行，而 Agent 自身的 root span 是在 `agent.Run()` 内部创建的。

```bash
export OPENAI_API_KEY="your-api-key"
export OPENAI_BASE_URL="https://api.openai.com/v1"
export LANGFUSE_SECRET_KEY="sk-lf-..."
export LANGFUSE_PUBLIC_KEY="pk-lf-..."
export LANGFUSE_HOST="localhost:3000"
export LANGFUSE_INSECURE="true"
go run . -session redis -enable-trace=true
```

## Session 命令

该示例支持以下 session 管理命令：

| 命令               | 描述                                         |
| ------------------ | -------------------------------------------- |
| `/new [id]`        | 创建新 session，可选指定自定义 ID             |
| `/sessions`        | 列出所有已知 session ID                      |
| `/use <id>`        | 切换到已有 session，或创建一个新的            |
| `/history`         | 让 assistant 回顾对话内容                    |
| `/search <query>`  | 后端支持时，召回相似事件                      |
| `/exit`            | 结束对话                                     |

## Session Management 工作流程

### 创建多个 Sessions

```
👤 You: Hello, I'm working on project A
🤖 Assistant: Hello! I'd be happy to help you with project A...

👤 You: /new
🆕 Started new session!
   Previous: chat-session-1703123456
   Current:  chat-session-1703123499
   (Conversation history has been reset)

👤 You: Hi, this is about project B
🤖 Assistant: Hello! Tell me about project B...
```

### 列出 Sessions

```
👤 You: /sessions
🗂 Session roster:
     chat-session-1703123456
   * chat-session-1703123499
```

`*` 表示当前活跃的 session。

### 切换 Sessions

```
👤 You: /use chat-session-1703123456
🔁 Switched to session chat-session-1703123456

👤 You: What were we discussing?
🤖 Assistant: We were talking about project A...
```

### 查看 Session 历史

```
👤 You: /history
🤖 Assistant: In our conversation so far, we discussed:
1. You mentioned working on project A
2. I offered to help with the project
...
```

## Session 存储后端

### In-Memory（默认）

- **适用场景**：开发、测试、快速演示
- **优点**：
  - 速度快
  - 无外部依赖
  - 零配置
- **缺点**：
  - 重启后数据丢失
  - 不适用于分布式系统
  - 仅限于单进程

### Redis

- **适用场景**：生产环境、分布式应用
- **优点**：
  - 持久化存储
  - 支持多实例
  - 自动 TTL/过期
  - 高性能
  - Pub/sub 能力
- **缺点**：
  - 需要 Redis 服务器
  - 额外的基础设施

### PostgreSQL

- **适用场景**：企业应用、复杂查询
- **优点**：
  - ACID 保证
  - 关系型数据模型
  - JSONB 存储，高效处理 JSON 操作
  - 支持软删除
  - 内置 TTL 清理
  - 丰富的查询能力
- **缺点**：
  - 需要 PostgreSQL 服务器
  - 占用更多资源

**PostgreSQL 特性：**
- JSONB 列用于 session 状态存储
- 软删除（数据标记为已删除，而非物理移除）
- 过期 session 的自动 TTL 清理
- session 重建的部分唯一索引
- 自动 schema 管理

### MySQL

- **适用场景**：基于 MySQL 的基础设施、遗留系统
- **优点**：
  - 广泛采用
  - 支持 JSON 存储
  - ACID 保证
  - 自动 TTL 清理
  - 兼容 MySQL 5.x+
- **缺点**：
  - 需要 MySQL 服务器
  - JSON 操作效率低于 PostgreSQL JSONB

**MySQL 特性：**
- JSON 列用于 session 数据
- 支持软删除
- 基于 TTL 的过期
- 自动 schema 创建
- 兼容 MySQL 5.7+

## 示例 Session

```
🚀 Session Management Demo
Model: deepseek-v4-flash
Streaming: true
Session Backend: PostgreSQL (localhost:5432/trpc-agent-go)
==================================================
✅ Chat ready! Session: chat-session-1703123456

💡 Session commands:
   /history   - Ask the assistant to recap our conversation
   /new       - Start a brand-new session ID
   /sessions  - List known session IDs
   /use <id>  - Switch to an existing (or new) session
   /exit      - End the conversation

👤 You: Hello! I'm planning a trip to Japan
🤖 Assistant: That's exciting! Japan is a wonderful destination...

👤 You: /new
🆕 Started new session!
   Previous: chat-session-1703123456
   Current:  chat-session-1703123500

👤 You: Hi, I need help with Python coding
🤖 Assistant: I'd be happy to help with Python!...

👤 You: /sessions
🗂 Session roster:
     chat-session-1703123456
   * chat-session-1703123500

👤 You: /use chat-session-1703123456
🔁 Switched to session chat-session-1703123456

👤 You: What were we talking about?
🤖 Assistant: We were discussing your trip to Japan...

👤 You: /exit
👋 Goodbye!
```

## 与 Runner 示例的关键区别

本 session 示例与基础 `examples/runner` 在以下几个方面的不同：

| 特性                      | examples/runner          | examples/session            |
| ------------------------- | ------------------------ | --------------------------- |
| **重点**                  | 基础 Runner 使用         | Session 管理                |
| **Session 后端**          | 仅 in-memory             | 多后端支持                  |
| **Session 命令**          | 无                       | /new、/sessions、/use       |
| **工具**                  | Calculator、Time         | 无（专注于 session）        |
| **复杂度**                | 最小                     | 高级                        |
| **使用场景**              | 快速上手、学习           | 生产模式                    |

## 帮助

查看所有可用的命令行选项：

```bash
go run . --help
```

## 后续步骤

探索完 session 管理之后：

1. **集成到你的应用**：在自有 agent 中使用 session service
2. **自定义存储**：配置 TTL、清理间隔、表名前缀
3. **添加认证**：实现基于用户的 session 隔离
4. **监控 sessions**：追踪活跃 session 和使用模式
5. **水平扩展**：使用 Redis/PostgreSQL 后端部署多实例
