# AG-UI Examples

本文件夹收集了可运行的 demos，展示了如何集成 `tRPC-Agent-Go` 的 AG-UI server 以及各种 clients。

- [`client/`](client/) – 通用的 client 侧示例。
- [`server/`](server/) – 通用的 server 侧示例。
- [`messagessnapshot/`](messagessnapshot/) – 展示如何启用和消费 message snapshots 的示例。

## Quick Start

1. 启动默认的 AG-UI server：

```bash
go run ./server/default
```

2. 在另一个终端启动 TDesign chat client：

```bash
cd ./client/tdesign-chat
pnpm install
pnpm dev
```

或者启动 CopilotKit client：

```bash
cd ./client/copilotkit
pnpm install
pnpm dev
```

3. 提一个问题，例如 `Calculate 2*(10+11)`，并在终端中查看实时的 event stream。在 [`client/copilotkit/README.md`](client/copilotkit/README.md) 中记录了一个完整的 transcript 示例。

关于更多背景信息和配置选项，请参阅 `client/` 和 `server/` 下的各个 README 文件。
