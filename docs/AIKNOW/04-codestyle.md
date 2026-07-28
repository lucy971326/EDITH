# EDITH Go 代码风格：显式生命周期的朴素服务端风格

> 给参与 EDITH 的 AI / 开发者的实现准则。
>
> 这套风格的名字可以叫：**显式生命周期 + 显式数据流（Explicit Lifetime, Explicit Data Flow）**。
> 它不是教条；当安全、资源释放、并发正确性或可读性明显要求另一种写法时，可以变通，但必须能说明原因。

---

## 1. 核心审美

EDITH 追求的不是“架构图看起来很专业”，而是让人从代码从上到下读下来就知道：

```text
什么能力长期存在？
本次请求输入了什么？
中间创建了什么资源？
它们在哪里释放？
数据最终流向哪里？
```

优先写这种代码：

```go
func (a *App) Run(ctx context.Context, userID, sessionID string, message model.Message) error {
	config, err := a.store.LoadUserConfig(ctx, userID)
	if err != nil {
		return err
	}

	tools, closeTools, err := a.loadMCPTools(ctx, userID, config.MCP)
	if err != nil {
		return err
	}
	defer closeTools()

	opts := []agent.RunOption{
		agent.WithRequestID(uuid.NewString()),
		agent.WithModelName(config.ModelName),
		agent.WithAdditionalTools(tools),
	}

	events, err := a.runner.Run(ctx, userID, sessionID, message, opts...)
	if err != nil {
		return err
	}
	return a.writeEvents(events)
}
```

这段代码的价值不在于短，而在于资源、参数和责任都没有躲起来。

## 2. 先判断“长期”还是“本次”

设计前先问：这个东西是长期拥有的能力/状态/身份，还是本次调用才存在的输入？

| 放在结构体字段 | 放在函数参数或局部变量 |
|---|---|
| 长期能力：Runner、SQLite、E2B client、HTTP client | `ctx`、`userID`、`sessionID`、用户消息 |
| 长期状态：缓存、Map、Mutex、长期配置 | 本次用户配置快照、RunOptions、Usage |
| 稳定身份：`appName`、基础目录、固定 Prompt | 本次 MCP ToolSet、工具列表、请求 ID |

禁止用 package 全局变量或 `App` 字段偷偷保存“当前用户”“当前会话”“当前请求的 sandbox”。这会破坏并发隔离，也让数据流不可读。

## 3. `var`、结构体字面量与 `New`

不要把 Go 写成“任何对象都必须 `NewXxx`”的 Java 风格。

### 默认选择：直接声明

纯数据、无副作用、无不变量维护时，优先使用 `var`、结构体字面量或普通函数：

```go
var edithRunner = runner.NewRunner("edith", chatAgent)

config := userconfig.Config{
	ModelName: "deepseek",
}

opts := []agent.RunOption{
	agent.WithStream(true),
}
```

`UserRuntime`、HTTP DTO、MCP 配置、RunOptions、一次 Run 的临时资源，通常都属于这一类。

### `New` 的正确理由

只有构造过程真的做事时才使用 `NewXxx`，例如：

- 打开 SQLite、创建 HTTP client、建立 E2B client；
- 校验必填配置或建立对象不变量；
- 初始化必须存在的 map、mutex、连接池；
- 启动后台 goroutine；
- 注册并接管一个需要 Close 的长期资源。

`New` 的名字应让读者意识到“这里发生了初始化或资源取得”，而不是单纯给结构体赋值。

## 4. 函数优先，不为流程制造对象

一次性或线性的流程，优先用函数，而不是套一个 `Manager`、`Builder`、`Factory`：

```go
func buildRunOptions(runtime UserRuntime, tools []tool.Tool) []agent.RunOption
func loadUserMCPTools(ctx context.Context, userID string, configs []MCPConfig) ([]tool.Tool, func(), error)
func writeSSE(w http.ResponseWriter, events <-chan *event.Event) error
```

当且仅当流程需要长期依赖、状态、缓存或多处复用时，才让它成为结构体的方法：

```go
type App struct {
	runner runner.Runner
	store  *sql.DB
	e2b    *e2b.Client
}
```

不要为了“分层”创建空壳 `Service`、`Repository`、`Manager`。名字不是架构；所有权和数据流才是。

## 5. 资源所有权必须靠代码看出来

谁创建资源，谁就应在同一可读范围内负责关闭，除非明确把所有权移交给调用方。

```go
tools, sets, err := loadUserMCPTools(ctx, userID, configs)
if err != nil {
	return err
}
defer closeToolSets(sets) // 在事件流消费完成后才返回
```

适用资源：MCP ToolSet、HTTP response body、数据库 rows、临时文件、sandbox 临时句柄、取消函数。

不要在资源加载函数内部 `defer Close()` 后再把资源返回；那会在调用者使用前关闭资源。

如果返回一个资源，就在命名或注释中说明 Close 责任，例如 `tools, closeTools, err := ...`。

## 6. 错误、ctx 与并发

- 所有可能阻塞的调用都接收并向下传递 `ctx`。
- 不吞错误；加必要上下文后返回，例如 `fmt.Errorf("load user MCP: %w", err)`。
- 不在没有必要时开 goroutine；一旦开 goroutine，必须能随 `ctx.Done()` 退出，并明确谁等待它结束。
- 不用全局可变状态存请求数据；并发下以参数传递或 session 持久化为准。
- 多返回值保持朴素：`value, err`、`value, cleanup, err` 通常比自造结果对象更清楚。

## 7. 拆文件的标准：按阅读路径拆，不按名词堆砌

一个文件过长、职责开始混杂或某段逻辑有独立复用价值时再拆。拆后仍要让主路径显眼：

```text
HTTP handler
  → App.Run
  → 读取用户配置 / 加载 MCP / 获取 Sandbox
  → Build RunOptions
  → Runner.Run
  → 事件转 SSE
  → 资源收尾
```

包内不需要为了“封装”把每个字段都私有化再造 getter。私有化应服务于不变量和防误用，而不是形式主义。

## 8. EDITH 特定约束

- 长期共享：Model、LLMAgent、Runner、SQLite、E2B client。
- 每次 Run 显式传递：`ctx`、`userID`、`sessionID`、消息、用户配置快照。
- 用户差异通过 RunOptions 表达；不要修改共享 Agent 的字段来服务某个用户。
- MCP ToolSet 是本次 Run 资源：创建 → 注入 `AdditionalTools` → 消费 Event 流 → Close。
- Sandbox 属于 `<userID, sessionID>`，由 EDITH + E2B SDK 管理，不使用框架 CodeExecutor。
- Event 流结束只认 `ev.IsRunnerCompletion()`；Usage 在每个事件中持续收集。

## 9. 允许变通，但要满足四个条件

下面情况可以不遵守“简单函数 / 直接 `var` / 就地收尾”的默认写法：

1. 能明显提升并发安全、资源释放正确性或安全边界；
2. 解决已经存在的重复，而不是假想的未来重复；
3. 新抽象比原本调用路径更容易理解；
4. 在代码注释或提交说明中写清楚为什么需要它。

例如：如果 E2B sandbox 需要带锁的 session 缓存和统一过期回收，一个长期 `SandboxService` 是合理的；它不是“过度设计”，因为它真实拥有长期状态与并发责任。

## 10. AI 提交前自检

- 我是否为了一个纯数据结构写了无意义 `NewXxx`？
- 我是否把一次请求的数据塞进了长期对象或全局变量？
- 谁创建了这个资源？谁在成功、失败、ctx 取消时关闭它？
- 主运行链是否能从一个函数顺着读下来？
- 新包 / 新接口是否解决了当前真实复杂度？
- 这段代码是否比直接写函数更清晰？若不是，删掉抽象。

当上述问题有不确定性，优先写更直接、资源边界更明显的版本。
