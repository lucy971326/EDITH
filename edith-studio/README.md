# EDITH Studio

本地单用户 Web Coding Agent。后端只监听 `127.0.0.1:8765`，浏览器通过 SSE 接收实时回复。

## 模型配置

创建 `%USERPROFILE%\.edith\models.yaml`：

```yaml
default: deepseek-flash

providers:
  deepseek:
    api_key: your-api-key
    base_url: https://api.deepseek.com
    variant: deepseek

models:
  deepseek-flash:
    provider: deepseek
    name: deepseek-v4-flash
    context_window: 1000000
    vision: false
    thinking:
      default: high
      modes: [disabled, low, high, max]
  deepseek-pro:
    provider: deepseek
    name: deepseek-v4-pro
    context_window: 1000000
    vision: false
    thinking:
      default: max
      modes: [disabled, low, high, max]
```

模型配置只从用户目录读取，不支持项目级覆盖。模型配置在启动时读取并创建实例；运行期间可以切换已注册模型，修改配置后重启 Studio 生效。所有允许字段以 [models/types.go](internal/models/types.go) 为准；未知字段会使启动失败。

## 本地运行

一个终端启动后端：

```powershell
go run ./cmd/edith
```

另一个终端启动 Web：

```powershell
Set-Location .\web
npm run dev
```

打开 http://localhost:3000。

## Langfuse 遥测（可选）

EDITH Studio 直接使用 tRPC-Agent-Go 内置的 Langfuse 遥测。把配置写入当前项目目录的 `.env` 后启动后端即可：

```dotenv
LANGFUSE_PUBLIC_KEY=pk-...
LANGFUSE_SECRET_KEY=sk-...
LANGFUSE_HOST=cloud.langfuse.com:443
```

然后运行：

```powershell
go run ./cmd/edith
```

本地自托管 Langfuse 可以使用 `LANGFUSE_HOST=localhost:3000`，并额外设置 `LANGFUSE_INSECURE=true`。系统环境变量优先于 `.env`；未配置这些变量时，Studio 会正常启动，但不会导出遥测数据。
