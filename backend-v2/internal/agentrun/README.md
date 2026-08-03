# agentrun

AgentRun 是 EDITH 的**唯一 Agent 执行入口**：接收 Gateway 已完成身份转换的请求，聚合配置，调用 ManagedRunner，输出中性事件流。

## 一张图看懂主链路

```text
Gateway / cronAdapter
        │ agentrun.Request
        ▼
Service.Run(request)
├─ normalize + validate
├─ sessionLanes.acquire(user, session, request)
│  └─ 已占用 → session_busy
├─ runConfigurations.Load(request)
│  ├─ Settings   → 默认模型 / personality
│  ├─ Models     → Definition + Model 能力
│  ├─ Providers  → API Key
│  ├─ MCP        → additional tools
│  ├─ Images     → 本次消息图片 URL
│  └─ Skills     → 公共摘要 + 当前用户 overview.md
│       ▼
│  configuredRun{Message, Options, Context, ...}
├─ Usage.Start
└─ ManagedRunner.Run(ctx, user, session, message, options)
       │ <-chan *event.Event
       ▼
  readFrameworkEvents（后台消费）
  ├─ agentstream.Decoder → 中性 agentstream.Event
  ├─ usage.AddTokens
  ├─ 输出 run.started / 内容 / 工具事件
  └─ runLifecycle.Finish
       ├─ Usage.Finish
       ├─ run.completed 或 run.canceled
       └─ MCP.Close + lane.release
       ▼
  Stream.Events ─────────────► Gateway / Adapter
```

## 对外结构

```text
Dependencies（main 创建 AgentRun 时传入）
├─ Runner    runner.ManagedRunner
├─ Models    *models.Catalog
├─ Settings  *userconfig.Settings
├─ Providers *userconfig.Providers
├─ MCP       *userconfig.MCP
├─ Images    *images.AgentInput
├─ Files     *sandbox.AgentInput
├─ Skills    *skills.Catalog
│  └─ 公共 Skill 目录 + 用户 overview 读取
└─ Usage     *usage.Recorder

Service（对外公开）
├─ Run(Request) → (*Stream, *Error)
├─ Status(userID, requestID) → (Status, *Error)
└─ Cancel(userID, requestID) → *Error

Request  = Gateway 翻译后的统一执行输入
Stream   = 唯一输出，Events 必须消费到关闭
Error    = { Type, Message }，用于启动和控制错误
Status   = { RequestID, Status }，仅表示活跃任务
```

## runConfigurations：配置聚合器

```text
runConfigurations
├─ models    *models.Catalog
├─ settings  *userconfig.Settings
├─ providers *userconfig.Providers
├─ mcp       *userconfig.MCP
├─ images    *images.AgentInput
└─ skills    *skills.Catalog
```

```text
Load(Request) → configuredRun
├─ 没指定 ModelID → Settings.LoadDefaultModelID
├─ Models.Find → Definition
├─ 校验图片模型是否支持 Vision
├─ Providers.LoadAPIKey
├─ Settings.LoadPersonality
├─ Images.AddMessageImages
├─ Files.ValidateUploads → 当前会话 uploads/ 普通文件
├─ MCP.OpenTools → tools + closeMCP
├─ Skills.ListSystemSummaries → 公共 Skill 摘要
├─ Skills.ReadUserOverview → 用户 Skill 摘要
├─ frameworkRunOptions → []agent.RunOption
└─ 返回完整 configuredRun
```

它负责“把四面八方配置聚合成一次可执行输入”，不调用 ManagedRunner。

## configuredRun：交给框架的完整运行包

```text
configuredRun
├─ ctx        context.Context
├─ request    Request
├─ definition models.Definition
│  ├─ 模型能力和用量统计规则
│  └─ ContextWindow → 摘要触发阈值
├─ run        usage.Run
├─ message    model.Message
├─ options    []agent.RunOption
└─ closeMCP   func() error
```

configuredRun.Close() → 释放本次 Load 打开的 MCP 工具连接。

## Stream：框架事件转换

```text
readFrameworkEvents
├─ 创建 agentstream.Decoder
├─ 先输出 run.started
├─ range rawEvents
│  ├─ Decoder.DecodeFrameworkEvent
│  ├─ 输出中性事件
│  ├─ 累加 Usage
│  └─ 收到 Completed → lifecycle.Finish
├─ rawEvents 结束后关闭输出 channel
└─ defer lifecycle.Close
```

```text
框架 *event.Event
        ↓ Decoder
中性 agentstream.Event
        ↓
Gateway / WebAdapter / cronAdapter
```

AgentRun 只负责消费和转发中性事件；Timeline 等展示模型属于前端或 Adapter。

## runLifecycle：一次运行的收尾责任

```text
runLifecycle
├─ service    *Service
├─ configured *configuredRun
└─ finished   bool
```

```text
Finish(tokens)
├─ Usage.Finish
├─ userStops.take(requestID)
├─ 生成 run.completed 或 run.canceled
└─ 返回带 SessionUsage 的终止事件

Close()
├─ 未正常 Finish → Usage.Fail
├─ configuredRun.Close → 关闭 MCP
├─ lanes.release → 允许下一条消息
└─ userStops.take → 清理取消标记
```

无论正常完成、取消、启动失败还是异常结束，资源释放都集中在这里。

## 会话并发与取消

```text
sessionLanes
└─ active map[userID + sessionID]requestID
   ├─ acquire → 抢占会话执行权
   └─ release → 任务结束后释放
```

```text
userStops
└─ requestID 集合
   ├─ mark → Cancel 成功前记录主动取消
   ├─ contains → 忽略取消产生的 runner error
   └─ take → 生成 run.canceled 并清理
```

Status 和 Cancel 都先由 ManagedRunner 校验 requestID 对应的活跃任务及用户归属，再执行操作。

## RunOptions：统一组装框架参数

```text
frameworkRunOptions(runOptionInput)
├─ RequestID
├─ Stream(true)
├─ ModelName
├─ ModelContextWindow
├─ Authorization header
├─ GlobalInstruction + personality
├─ 一次 WithInstruction：公共摘要 + 用户 overview + 资源路径说明 + 本次已验证上传文件
└─ additionalTools（MCP）
```

runOptionInput 只存在于 AgentRun 内部；外部调用方只提交 Request。

## 会话摘要

```text
main
├─ agentrun.NewSessionSummarizer()
│  └─ summary.NewDynamicSummarizer
└─ sessionsqlite.NewService(..., WithSummarizer(...))

每次 AgentRun
├─ RunOptions.ModelContextWindow = 当前模型窗口
├─ RunOptions.ModelRequestHeaders = 当前用户 API Key
└─ 新增事件达到窗口 40% → 框架异步生成摘要
       ├─ 摘要模型 = 当前 Invocation.Model
       ├─ 摘要请求复用当前请求头
       ├─ 原始 Events 继续保留
       └─ 后续请求 = Summary + 摘要后的新事件
```

摘要模型只存在于本次摘要调用中；API Key 不写入 SQLite，也不进入摘要正文。摘要失败由框架记录并保留原始事件，下一次仍可重试。

## 一句话记忆

```text
Gateway   = 把渠道请求翻译成 Request
AgentRun  = 加载配置 + 组装 Options + 调 ManagedRunner
Decoder   = 框架事件 → 中性事件
Stream    = AgentRun 唯一输出
Lifecycle = 用量、MCP、并发锁、取消标记的统一收尾
```
