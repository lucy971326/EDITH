# models

模型目录模块。它不运行 Agent，也不保存用户密钥，只负责三件事：

```text
声明 EDITH 支持的供应商和模型
        │
        ├─► 给 HTTP 返回浏览器需要的模型信息
        ├─► 给 AgentRun 查找模型定义
        └─► 给 ManagedRunner 注册真实模型适配器
```

## 模块整体结构

```text
models.New(Dependencies)
        │
        ▼
     Module
     ├─ Catalog ─────► AgentRun / ManagedRunner
     └─ HTTP ─────────► Web BFF
                         ├─ GET /internal/models
                         └─ GET /internal/available-models
```

`main` 只负责创建模块、取出能力并注册路由：

```text
main
├─ users := userconfig.New(...)
├─ modelModule := models.New(models.Dependencies{
│      Providers: users.Providers,
│  })
├─ modelModule.Catalog.Registered() ─► ManagedRunner
├─ modelModule.Catalog              ─► AgentRun
└─ modelModule.HTTP.Register(mux)   ─► HTTP 路由
```

## 结构体扁平展开

### Dependencies：创建模块时需要的外部能力

```text
Dependencies
└─ Providers *userconfig.Providers
   └─ 用于 HTTP 查询某个用户是否配置了供应商 API Key
```

它只在 `models.New` 时使用，不是模型目录本身的数据。

### Module：模块对外公开的两扇门

```text
Module
├─ Catalog *Catalog
│  └─ 给 Go 模块调用
└─ HTTP *HTTP
   └─ 给 Web BFF 调用
```

`Module` 不承载业务逻辑，只把模型模块的两类公开能力放在一起。

### Catalog：模型清单的内存目录

```text
Catalog
├─ providers []ProviderInfo
│  └─ 供应商清单
└─ definitions []Definition
   └─ 具体模型清单
```

`Catalog` 只保存应用启动时声明的固定清单，不访问数据库。

它的方法分别服务不同调用方：

```text
Catalog.Providers()
└─ 返回 []ProviderInfo
   服务：设置页 / 模型目录 HTTP

Catalog.Models()
└─ 返回 []Info
   服务：浏览器模型选择器
   只暴露安全的前端模型描述

Catalog.Find(modelID)
└─ 返回 Definition
   服务：AgentRun
   按 EDITH 模型 ID 找到完整运行定义

Catalog.Registered()
└─ 返回 map[string]model.Model
   服务：main 创建 ManagedRunner
   把真实 Go 模型适配器注册进去
```

## 目录中的四种资料

### ProviderInfo：供应商视图

```text
ProviderInfo
├─ ID   string  供应商稳定 ID，例如 minimax
└─ Name string  给浏览器显示的名称，例如 MiniMax
```

服务对象：设置页和模型目录接口。

它回答的是：“用户要给哪一家供应商配置 API Key？”

```text
MiniMax（供应商）
  ├─ MiniMax M3
  └─ 未来其他模型
```

### ReasoningOption：模型专属推理选项

```text
ReasoningOption
├─ ID   string  选项稳定 ID
└─ Name string  浏览器显示名称
```

它是 `Info` 的子结构，只服务模型选择器显示和提交推理选项；当前某些模型没有选项时就是空数组。

### Capabilities：模型能力标记

```text
Capabilities
└─ Vision bool  是否支持图片输入
```

它是 `Info` 的子结构，服务前端判断是否显示图片能力，也供 AgentRun 校验输入是否匹配模型能力。

### Info：浏览器模型视图

```text
Info
├─ ID               string
│  └─ EDITH 稳定模型 ID
├─ ProviderID       string
│  └─ 归属哪个 ProviderInfo
├─ Name             string
│  └─ 浏览器显示名称
├─ ReasoningOptions []ReasoningOption
│  └─ 模型可选推理配置
└─ Capabilities     Capabilities
   └─ vision 等能力标记
```

`Info` 可以安全序列化为 JSON 返回浏览器，不包含 API Key、HTTP 客户端或模型适配器。

### Definition：服务端运行视图

```text
Definition
├─ Info
│  └─ 前端可见的模型描述
├─ Model model.Model
│  └─ trpc-agent-go 真正调用的模型适配器
└─ DoesNotReportCachedPromptTokens bool
   └─ 用量统计的特殊规则
```

它回答的是：“AgentRun 要用哪个真实模型对象执行？”

因此 `Definition` 比 `Info` 多了服务端私有内容；`Model` 绝不直接返回给浏览器。

## 为什么 ProviderInfo 和 Definition 都需要

两者不是同一层：

```text
ProviderInfo                    Definition
“供应商是谁？”                  “具体运行哪个模型？”
├─ minimax                      ├─ ID: minimax.m3
└─ 一把 API Key                  ├─ Info: 浏览器描述
                                ├─ Model: 运行时适配器
                                └─ 用量规则
```

一个供应商可以拥有多个模型。设置页关心供应商凭据，Runner 关心具体模型；拆开后两个使用者都不需要理解对方的数据。

## HTTP 结构体与接口

### HTTP：模型模块的 Web 入口

```text
HTTP
├─ catalog   *Catalog
│  └─ 读取固定模型目录
└─ providers *userconfig.Providers
   └─ 查询用户凭据状态
```

`HTTP` 只做 HTTP/JSON 与 Go 能力之间的翻译，不创建模型、不保存密钥。

```text
HTTP.Register(mux)
└─ 由 main 显式调用，注册本模块路由
```

### GET /internal/models：完整公共目录

```text
请求
  ↓
HTTP.listCatalog
  ├─ Catalog.Providers() → []ProviderInfo
  └─ Catalog.Models()    → []Info
  ↓
CatalogResponse
├─ Providers []ProviderInfo
└─ Models    []Info
```

服务：模型设置页、模型选择器。

### GET /internal/available-models：当前用户可用模型

```text
请求 ?userId=clerk_user_id
  ↓
HTTP.listAvailable
  ├─ userconfig.Providers.ListStatuses(userID)
  ├─ Catalog.Models()
  └─ 过滤出已配置 API Key 的模型
  ↓
AvailableResponse
└─ Models []Info
```

服务：聊天页模型选择器。

## 一句话记忆

```text
ProviderInfo = 配哪家的 Key
Info         = 浏览器显示什么
Definition   = Agent 实际运行什么
Catalog      = 把这些固定资料放在一起查询
HTTP         = 把目录翻译成 Web JSON
```
