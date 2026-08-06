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

models:
  deepseek-flash:
    provider: deepseek
    name: deepseek-v4-flash
  deepseek-pro:
    provider: deepseek
    name: deepseek-v4-pro
```

模型配置只从用户目录读取，不支持项目级覆盖。所有允许字段以 [models/types.go](internal/models/types.go) 为准；未知字段会使启动失败。

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
