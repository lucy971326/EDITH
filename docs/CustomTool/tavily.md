> ## 文档索引
> 获取完整文档索引：https://docs.tavily.com/llms.txt
> 使用此文件可发现所有可用页面，然后再进一步探索。

# Tavily MCP Server

> Tavily MCP Server 允许你在 MCP 客户端中使用 Tavily API。

## 远程 MCP Server

使用 Tavily MCP 最简单的方式是通过远程 URL。无需本地安装或配置，即可获得无缝体验。

只需将远程 MCP server URL 与你的 Tavily API key 一起使用：

```
https://mcp.tavily.com/mcp/?tavilyApiKey=<your-api-key>
```

从 [tavily.com](https://www.tavily.com/) 获取你的 Tavily API key。

### 连接到 Cursor

点击上方按钮将你重定向到 Cursor 的 `mcp.json` 文件，你需要在该文件中添加你的 API key。

```json
{
  "mcpServers": {
    "tavily-remote-mcp": {
      "command": "npx -y mcp-remote https://mcp.tavily.com/mcp/?tavilyApiKey=<your-api-key>",
      "env": {}
    }
  }
}
```

### 连接到 Claude Desktop

Claude Desktop 现在支持添加 `integrations`（目前处于 beta 阶段）。打开 Claude Desktop，点击设置按钮，然后导航到添加 integrations。命名该 integration 并插入带有 API key 的 Tavily 远程 MCP URL。点击 `Add` 确认。

### OpenAI

允许模型使用远程 MCP server 执行任务。

- 首先需要导出你的 `OPENAI_API_KEY`
- 同时需要在 `<your-api-key>` 中添加你的 Tavily API key

```python
from openai import OpenAI
import json

client = OpenAI()

resp = client.responses.create(
    model="gpt-4.1",
    tools=[
        {
            "type": "mcp",
            "server_label": "tavily",
            "server_url": "https://mcp.tavily.com/mcp/?tavilyApiKey=<your-api-key>",
            "require_approval": "never",
            "headers": {
                "DEFAULT_PARAMETERS": json.dumps({
                    "include_favicon": True,
                    "include_images": False,
                    "include_raw_content": False,
                }),
            },
        },
    ],
    input="Do you have access to the tavily mcp server?",
)

print(resp.output_text)
```

### 连接到 Claude Code

[Claude Code](https://docs.anthropic.com/en/docs/claude-code) 原生支持通过 OAuth 认证的远程 MCP server。运行以下命令将 Tavily 添加到 Claude Code 配置：

```bash
claude mcp add tavily-remote-mcp --transport http https://mcp.tavily.com/mcp/
```

当你开始新对话时，Claude Code 会打开浏览器窗口完成 OAuth 流程并授权访问你的 Tavily 账户。URL 中无需 API key——认证通过 OAuth 自动处理。

你也可以手动将以下内容添加到 `.claude/settings.json`：

```json
{
  "mcpServers": {
    "tavily-remote-mcp": {
      "type": "http",
      "url": "https://mcp.tavily.com/mcp/"
    }
  }
}
```

或者，你也可以使用 `mcp-remote` 连接：

```bash
claude mcp add tavily-remote-mcp -- npx -y mcp-remote https://mcp.tavily.com/mcp
```

### 不支持远程 MCP 的客户端

`mcp-remote` 是一个轻量级桥接工具，让只能与本地（stdio）server 通信的 MCP 客户端能够通过 HTTP + SSE 配合 OAuth 认证安全地连接到远程 MCP server。这样你可以在云端托管和更新 server，同时保持现有客户端正常工作。它作为一项实验性过渡方案，直到主流 MCP 客户端原生支持远程、带认证的 server。

```json
{
  "tavily-remote": {
    "command": "npx",
    "args": [
      "-y",
      "mcp-remote",
      "https://mcp.tavily.com/mcp/?tavilyApiKey=<your-api-key>"
    ]
  }
}
```

### OAuth 认证

Tavily 远程 MCP server 支持安全的 OAuth 认证，让你能够与兼容的客户端无缝连接和授权。

#### 使用 MCP Inspector

打开 MCP Inspector 并点击 "Open Auth Settings"。选择 OAuth 流程并完成以下步骤：

1. Metadata discovery
2. Client registration
3. Preparing authorization
4. 请求授权并获取 authorization code
5. Token request
6. 认证完成

完成后，你将收到一个 access token，用于安全地向 Tavily 远程 MCP server 发起认证请求。

#### 使用其他 MCP 客户端

你可以在不将 Tavily API key 包含在 URL 中的情况下配置 MCP 客户端使用 OAuth。例如，在 Cursor 的 `mcp.json` 中：

```json
{
  "mcpServers": {
    "tavily-remote-mcp": {
      "command": "npx mcp-remote https://mcp.tavily.com/mcp",
      "env": {}
    }
  }
}
```

如果需要清除已存储的 OAuth 凭据并重新认证，运行：

```bash
rm -rf ~/.mcp-auth
```

> **OAuth 的 API Key 选择**
>
> 使用 OAuth 认证时，你可以在 Tavily dashboard 中将某个 key 命名为 `mcp_auth_default` 来控制使用哪个 API key：
>
> - **个人账户**：如果你的个人账户中有名为 `mcp_auth_default` 的 key，它将用于所有 OAuth 认证的请求。
> - **团队账户**：如果你的团队有名为 `mcp_auth_default` 的 key，它将用于所有 OAuth 认证的请求。
> - **两者都设置**：如果个人账户和团队都设置了 `mcp_auth_default`，**个人 key 优先**。
> - **均未设置**：如果没有 `mcp_auth_default` key，则使用个人账户中的 `default` key。如果没有设置 `default` key，则使用第一个可用的 key。
>
> OAuth 认证是可选的——你仍然可以随时通过在 URL 查询参数中包含 Tavily API key（`?tavilyApiKey=...`）或在 Authorization header 中设置它来使用 API key 认证。

你也可以选择在本地运行 MCP server。

### 默认参数

使用远程 MCP 时，你可以通过在 `DEFAULT_PARAMETERS` header 中包含一个 JSON 对象来为所有请求指定默认参数。示例：

```
{"include_images":true, "search_depth": "advanced", "max_results": 10}
```

### Session 与用户归属

远程 MCP server 会自动为每个 Tavily API 调用附加标识符，以便将请求归属于某个 session：

- **`X-Session-Id`**——每个 MCP session 自动生成（MCP `initialize` 握手期间返回的 `mcp-session-id`）。同一 MCP session 内的所有 tool 调用共享相同的值。
- **`X-Human-Id`**——如果你的客户端提供了 `X-Human-Id` header（或 MCP URL 上的 `humanId` query parameter），该值会被转发到 Tavily API，帮助 Tavily 更好地理解多步交互并提高响应质量。出于安全考虑，Tavily 在处理或存储 human ID 之前会对其进行哈希处理。

## 本地安装

### 前提条件

- [Tavily API key](https://app.tavily.com/home)
  - 如果你没有 Tavily API key，可以在[此处](https://app.tavily.com/home)注册免费账户
- [Claude Desktop](https://claude.ai/download) 或 [Cursor](https://cursor.sh)
- [Node.js](https://nodejs.org/)（v20 或更高版本）
  - 你可以通过运行以下命令验证 Node.js 安装：
    ```bash
    node --version
    ```

#### Git 安装（可选）

仅在使用 Git 安装方式时需要：

- macOS：`brew install git`
- Linux：
  - Debian/Ubuntu：`sudo apt install git`
  - RedHat/CentOS：`sudo yum install git`
- Windows：下载 [Git for Windows](https://git-scm.com/download/win)

**NPX 方式：**

```bash
npx -y tavily-mcp@0.1.3
```

**Git 方式：**

```bash
git clone https://github.com/tavily-ai/tavily-mcp.git
cd tavily-mcp
npm install
npm run build
```

> 虽然你可以独立启动 server，但单独使用意义不大。相反，你应该将其集成到 MCP 客户端中。

### 配置 MCP 客户端

#### Cursor

> **注意**：需要 Cursor 0.45.6 或更高版本

在 Cursor 中设置 Tavily MCP server：

1. 打开 Cursor Settings
2. 导航到 Features > MCP Servers
3. 点击 "+ Add New MCP Server" 按钮
4. 填写以下信息：
   - **Name**：为 server 输入一个昵称（例如 "tavily-mcp"）
   - **Type**：选择 "command"
   - **Command**：输入运行 server 的命令：
     ```bash
     env TAVILY_API_KEY=tvly-YOUR_API_KEY npx -y tavily-mcp@0.1.3
     ```
     > 将 `tvly-YOUR_API_KEY` 替换为你的 Tavily API key，可从 [app.tavily.com/home](https://app.tavily.com/home) 获取

#### Claude Desktop

**macOS：**

```bash
# 创建配置文件（如果不存在）
touch "$HOME/Library/Application Support/Claude/claude_desktop_config.json"

# 在 TextEdit 中打开配置文件
open -e "$HOME/Library/Application Support/Claude/claude_desktop_config.json"

# 使用 Visual Studio Code 的替代方法
code "$HOME/Library/Application Support/Claude/claude_desktop_config.json"
```

**Windows：**

```bash
code %APPDATA%\Claude\claude_desktop_config.json
```

添加以下配置（将 `tvly-YOUR_API_KEY-here` 替换为你的 [Tavily API key](https://tavily.com/api-keys)）：

```json
{
  "mcpServers": {
    "tavily-mcp": {
      "command": "npx",
      "args": ["-y", "tavily-mcp@0.1.2"],
      "env": {
        "TAVILY_API_KEY": "tvly-YOUR_API_KEY-here"
      }
    }
  }
}
```

### 默认参数

对于本地 MCP 设置，你可以使用 `DEFAULT_PARAMETERS` 环境变量设置默认参数值。这样可以在不每次请求都指定这些参数的情况下配置默认搜索行为。

```json
{
  "mcpServers": {
    "tavily-mcp": {
      "command": "npx",
      "args": ["-y", "tavily-mcp@latest"],
      "env": {
        "TAVILY_API_KEY": "your-api-key-here",
        "DEFAULT_PARAMETERS": "{\"include_images\": true, \"max_results\": 15, \"search_depth\": \"advanced\"}"
      }
    }
  }
}
```

### Session 与用户归属

本地 MCP server 会自动为每个 Tavily API 调用附加标识符，以便将请求归属于某个 session：

- **`X-Session-Id`**——每个 MCP 进程自动生成一次，并在所有 tool 调用中重复使用。
- **`X-Human-Id`**——如果你设置了 `HUMAN_ID` 环境变量，该值会在每次请求时被转发到 Tavily API，帮助 Tavily 更好地理解多步交互并提高响应质量。出于安全考虑，Tavily 在处理或存储 human ID 之前会对其进行哈希处理。

## 使用示例

### Tavily Search 示例

1. **通用 Web 搜索**：

   ```
   Can you search for recent developments in quantum computing?
   ```

2. **新闻搜索**：

   ```
   Search for news articles about AI startups from the last 7 days.
   ```

3. **指定域名搜索**：

   ```
   Search for climate change research on nature.com and sciencedirect.com
   ```

### Tavily Extract 示例

**提取文章内容**：

```
Extract the main content from this article: https://example.com/article
```

### 组合使用

```
Search for news articles about AI startups from the last 7 days and extract the main content from each article to generate a detailed report.
```

## 故障排除

### Server 未找到

如果遇到 server 连接问题，运行以下命令验证你的环境：

```bash
npm --version
node --version
```

同时检查你的配置语法是否有任何错误。

### NPX 问题

如果使用 npx 时遇到问题，找到你的可执行文件路径：

```bash
which npx
```

> 获取路径后，更新你的配置以使用 npx 可执行文件的完整路径。

### API Key 问题

排查 API key 问题时，请验证你的 key：

- 格式正确，带有 `tvly-` 前缀
- 在 Tavily dashboard 中有效且处于活跃状态
- 在环境变量中正确配置

> 你可以通过 [Tavily Playground](https://app.tavily.com/playground) 发送简单的测试请求来验证 API key 的有效性。

## 致谢

- [Model Context Protocol](https://modelcontextprotocol.io)——MCP 规范
- [Anthropic](https://www.anthropic.com/claude)——Claude Desktop
