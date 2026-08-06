# OpenClaw 多模型兼容与注册机制调研(1.10 → 1.11)

> 面向开发者与 AI 的完整调研结论。数据来源为 `reference/trpc-agent-go/openclaw` 与 `reference/trpc-agent-go/model` 源码 + 官方文档(`docs/mkdocs/zh/model.md`、`openclaw/README.md`),证据索引见文末。
> 调研方式:配置 → 源码 → 注册表 → 调用链,逐层确认。
> 覆盖版本:1.10.0(注册机制基线)→ 1.11.0(Model Catalog 结构升级)。

---

## 1. 一句话结论

**OpenClaw 的多模型 = 一套 OpenAI 兼容协议 + 7 个 provider 变体**,兼容 7 家(OpenAI / DeepSeek / 通义千问 / 混元 / 智谱GLM / MiniMax / Kimi),外加一条驱动 Claude Code CLI 的通路(`agent.type: claude-code`)。

而**接其他模型的注册机制**,分两个阶段:

> - **1.10**:一张带锁的 map + `(typeName, factory)` 注册 API + `ModelSpec` 契约,编译期注册、运行期按 `model.mode` 选择。它不是动态加载器,是一个插件接缝。
> - **1.11**:从"单模型"升级为"**模型目录(Model Catalog)**"——一个 runtime 预注册多个模型(按别名),请求时按别名动态选。三层协同。

---

## 2. 三层结构总览

```
┌─ 第 3 层:agent 类型(执行后端,不是模型)
│     llm(默认) / claude-code(驱动 Claude Code CLI)
│
├─ 第 2 层:model/openai —— OpenAI 兼容协议 + openai_variant(7 家)
│
└─ 第 1 层:模型注册与选择
│     · 1.10:registry.RegisterModel 单一注册
│     · 1.11:Model Catalog(多模型目录)在上层叠加
```

| 层 | 1.10(旧) | 1.11(新) |
|---|---|---|
| 框架层 `llmagent` | `WithModel(单个)` | 新增 `WithModels(map)` 预注册多个 + `WithModelName` 请求级选 |
| openclaw 层 | `model.mode` → 一个模型 | **Model Catalog**:`model.models` + `model.default` 多模型目录 |
| `model/openai` | 7 家 variant | 仍是 7 家 + 各家适配修复 |

**不是 7 套独立实现**,而是基于 OpenAI Chat Completions 协议 + 每家一个变体配置。这是理解全部机制的前提。

---

## 3. 第一层:注册机制(1.10 基线)

### 3.1 数据层面:一张带锁的 map

`registry/registry.go:214-224` 定义了 7 类并排的注册表,**模型只是其中一类**,7 类共用同一套 Register/Lookup 模式:

```go
var (
    mu sync.RWMutex
    ...
    modelFactories = map[string]ModelFactory{}   // ← 模型注册表
)
```

| 操作 | 函数 | 说明 |
|---|---|---|
| 写 | `RegisterModel(typeName, factory)` | 校验 type 名(小写/去空格/非空)→ 校验 factory 非 nil → **查重**(重复直接报错)→ 插入。全程加锁 |
| 读 | `LookupModel(typeName)` | 规范化名字后读 map,返回 `(factory, bool)` |

**没有任何 `.so` 动态加载、没有配置驱动的插件实例化、没有运行期热注册**。这是最关键的一条认知。

### 3.2 契约层面:三个类型构成"接缝"

```go
// ① 要什么 —— "要创建哪个模型"的完整描述
type ModelSpec struct {            // registry.go:196
    Type          string           // 注册的 typeName
    Name          string           // 模型名
    BaseURL       string
    APIKey        string
    OpenAIVariant string           // 仅 openai 用
    Timeout       time.Duration
    MaxRetries    *int
    Headers       map[string]string
    Config        *yaml.Node       // ← 自由格式逃生舱
}

// ② 怎么建 —— 工厂函数(插件要实现)
type ModelFactory func(spec ModelSpec) (model.Model, error)   // registry.go:212

// ③ 建出什么 —— 只有 2 个方法的接口
type Model interface {             // model/model.go
    GenerateContent(ctx, *Request) (<-chan *Response, error)
    Info() Info
}
```

**`ModelSpec` 是所有模型配置的唯一漏斗**:框架层把任何模型的配置收拢成这一份 spec 传给工厂。`Config *yaml.Node` 是逃生舱——`model.config:` 下任意自定义子结构原样塞进来,插件用 `registry.DecodeStrict`(registry.go:509,`KnownFields(true)` 严格解析)自己解码。

### 3.3 内置注册内容

`app/builtins.go:16-17` 在 `init()` 中只注册了 2 个(1.10 与 1.11 均如此):

```go
must(registry.RegisterModel(modeMock, newMockModel))
must(registry.RegisterModel(modeOpenAI, newOpenAIModel))
```

- `mock` — 测试假模型(工厂只有一行,返回 echo 模型,`app.go:3807`)
- `openai` — OpenAI 兼容模型,内部再按 variant 分派 7 家(`app.go:3840`)

`model.mode`(CLI `-mode`)只有这两个合法值,默认 `openai`(`run_options.go:647-652`)。框架 `model/` 包里虽有 anthropic / gemini / bedrock / ollama / hunyuan 等独立实现,但 **OpenClaw 官方一个都没注册**。

### 3.4 完整调用链(1.10 单模型路径)

```
openclaw.yaml                     CLI flag                  env
  model.mode: "openai"      ───▶  -mode openai
  model.name: "gpt-5"       ───▶  -model gpt-5              OPENAI_MODEL
  model.base_url: ...       ───▶  -openai-base-url          OPENAI_BASE_URL
  model.openai_variant      ───▶  -openai-variant           OPENAI_API_KEY
  model.config: {...}       ───▶  opts.ModelConfig (raw yaml.Node)
        │
        ▼
 [modelFromOptions  app.go:3898]   ← 消费端
    mode := opts.ModelMode          // 默认 "openai"
    f, ok := registry.LookupModel(mode)   // ← 按名字查注册表
    if !ok { return nil, fmt.Errorf("unsupported mode: %s", mode) }
    // 组装 ModelSpec(openai 模式下 baseURL 兜底读 OPENAI_BASE_URL、
    //  apiKey 读 OPENAI_API_KEY、headers 走 resolveOpenAIHeaders)
    spec := registry.ModelSpec{ Type: mode, Name: opts.OpenAIModel, ... }
    mdl, err := f(spec)             // ← 调注册进来的工厂
    return newModelTimeoutModel(mdl, timeout)
        │
        ▼
  这个 model.Model 交给 runner.WithModel(...) 建 agent
```

- yaml → opts 的映射:`run_options.go:1924-1975`;`modelConfig` 结构体:`run_options.go:1307-1317`
- 内置 openai 工厂:`newOpenAIModel`(app.go:3840)—— 解析 variant → 攒 `[]openai.Option` → `openai.New(name, opts...)`
- 变体自动推断:`openai_variant: "auto"` 按 base_url 的 host 判断(`inferOpenAIVariant`,app.go:4172-4195)

---

## 4. 第二层:7 家 provider 变体(两版通用)

`model/openai/openai.go:78-96` 定义 7 个 `Variant`;每家的差异适配在 `variantConfigs` 表(openai.go:271-398)。

| Variant | 官方 base_url | API Key 环境变量 | 特殊适配 |
|---|---|---|---|
| `openai` | (自定义) | OPENAI_API_KEY | 默认;缓存优化 |
| `deepseek` | api.deepseek.com | DEEPSEEK_API_KEY | thinking 用 `{"type":"enabled"}` 格式;推理回放自动补 `reasoning_content`;消息只发纯文本 |
| `qwen` | dashscope.aliyuncs.com/compatible-mode/v1 | DASHSCOPE_API_KEY | thinking 用 `enable_thinking` 键 |
| `hunyuan` | api.hunyuan.cloud.tencent.com | — | 文件上传走特殊 multipart 格式;thinking 用 type 格式 |
| `glm` | open.bigmodel.cn | — | `reasoning_content` 空 content 回退;thinking type 格式;OpenClaw 额外给它开 tool call 参数修复(app.go:4121-4135) |
| `minimax` | api.minimax.io / api.minimaxi.com | MINIMAX_API_KEY | thinking 用 `{"type":"adaptive"}`;文件删除格式特殊 |
| `kimi` | api.moonshot.ai / api.moonshot.cn | MOONSHOT_API_KEY | 文件 purpose 用 `file-extract`;thinking type 格式 |

**各家差异适配点**(variantConfigs 字段):thinking 开关报文格式 / 文件上传删除 API 路径与报文 / `reasoning_content` 处理 / 消息内容结构。

关键体验:**`openai_variant: auto` 按 base_url 的 host 自动推断**,用户填对 URL + Key 就自动切到对应 provider 适配。

> 注意:**Requesty 不是新 variant**。1.11 changelog 里的 "Add Requesty support" 是框架级 example(`examples/model/requesty/main.go`),走 OpenAI 兼容路径 + 自定义 base_url 即可,不需要新 variant。

---

## 5. 第三层:agent 类型(两版通用)

`agent.type` 支持 `llm`(默认)与 `claude-code`(驱动本机 Claude Code CLI 跑消息)。

- `normalizeAgentType`(app.go:2597-2610)校验
- claude-code 模式不支持 tools 配置、session-summary、ralph loop 等(run_options.go 校验)
- **注意:claude-code 不是模型 provider,是另一套执行后端**——Claude 模型走 CLI,不走 `model/anthropic`

---

## 6. 1.11 结构升级:Model Catalog(核心新增)

### 6.1 新增文件与数据结构

新增 `app/model_catalog.go`。核心结构:

```go
// model_catalog.go:27
type resolvedModelCatalog struct {
    defaultName string                 // 默认别名
    models      map[string]model.Model // 多个模型实例,按别名
    metadata    map[string]ModelMetadata // 每别名的 mode/variant/baseURL
    explicit    bool                   // 是否为显式目录
}

// runtime_options.go:85 —— 每别名的元数据
type ModelMetadata struct {
    Mode          string
    BaseURL       string
    BaseURLSet    bool   // 空 BaseURL 覆盖(而非继承)顶层 base_url
    OpenAIVariant string
}

// runtime_options.go:92 —— 嵌入式注入目录
type ModelCatalog struct {
    Default  string
    Models   map[string]model.Model
    Metadata map[string]ModelMetadata
}

// run_options.go —— yaml 单 entry 配置
type modelEntryConfig struct {   // yaml: model.models.<alias>
    Mode            *string
    Name            *string
    BaseURL         *string
    OpenAIVariant   *string
    TextOnlyContent *bool
    Timeout         *string
    MaxRetries      *int
    Headers         map[string]string
    Config          *rawYAMLNode
}
```

### 6.2 三种来源(resolveModelCatalog:51)

| 来源 | 入口 | 说明 |
|---|---|---|
| **YAML 配置** | `model.models` + `model.default` | 配置文件预注册多个模型 |
| **嵌入式注入** | `WithModelCatalog(ModelCatalog{...})` | Go 代码直接塞构造好的模型实例(企业分发) |
| **legacy 回退** | 不配 → `modelFromOptions` | 回到 1.10 单模型路径 |

三者互斥:YAML `model.models` 与 `WithModelCatalog` **不能同时用**,冲突直接报错(model_catalog.go:55)。

### 6.3 YAML 配置格式(README:841)

```yaml
model:
  default: balanced                      # 默认别名
  models:
    balanced:
      mode: openai
      name: deepseek-v4-pro
      openai_variant: deepseek
      base_url: "${DEEPSEEK_BASE_URL}"
    fast:
      mode: openai
      name: deepseek-v4-flash
      openai_variant: deepseek
      base_url: "${DEEPSEEK_BASE_URL}"
    strong:
      mode: openai
      name: gpt-5
      openai_variant: openai
      base_url: "${OPENAI_BASE_URL}"
```

每个 entry 通过 `modelRunOptionsForEntry`(model_catalog.go:293)转成独立的 runOptions,再各自 `modelFromOptions` 创建。**每个模型独立配置、独立创建、互不污染**。

### 6.4 选择逻辑(selectedModelName:368)

```
请求带的 model 别名 → runtime profile 的 model_name → model.default
```

Gateway 层集成(`appendModelCatalogGatewayOptions`:436):暴露 `WithSelectableModels` + `WithRunOptionResolver`,请求 JSON 里带 `"model": "strong"` 就选 strong:

```json
{ "from": "user-1", "session_id": "conversation-1", "text": "...", "model": "strong" }
```

**关键行为(README:885):未知别名直接 HTTP 400(`error.type = "invalid_model"`),绝不静默回退默认。** 显式请求选择优先于 runtime profile 的 `model_name`。

### 6.5 1.11 的新细节

- **API key 按 variant 取**:每个 entry 用 variant 专属 env(`DEEPSEEK_API_KEY` / `DASHSCOPE_API_KEY` / `MINIMAX_API_KEY` / `MOONSHOT_API_KEY`),没有才 fallback `OPENAI_API_KEY`(README:868)。对应新字段 `OpenAIUseVariantAPIKey`(model_catalog.go:303 默认 true)。
- **每个模型独立 call budget / deadline 收尾**:`appendModelCallBudgetGatewayOption`:470 按选中模型算各自的预算和 final request 配置。
- **每个模型独立 compatibility run option**:GLM 才开 tool call 修复,按选中模型动态决定(app.go:4321)。
- **仅作用于当前主 agent run**:会话摘要、自动记忆、标题、子 agent 仍用各自配置(README:889)。
- **仅 `agent-type: llm` 支持** catalog(validateModelCatalogAgentType:34)。
- **runtime instance id 指纹**:catalog 参与进程指纹计算,不同目录 = 不同实例(app.go:2685)。

### 6.6 嵌入式注入示例(README:894)

```go
rt, err := app.NewRuntimeWithOptions(
    ctx, args,
    app.WithModelCatalog(app.ModelCatalog{
        Default: "balanced",
        Models: map[string]model.Model{
            "balanced": balancedModel,
            "strong": strongModel,
        },
        Metadata: map[string]app.ModelMetadata{
            "balanced": { Mode: "openai", OpenAIVariant: "deepseek", BaseURL: "https://api.deepseek.com/v1" },
            "strong":   { Mode: "openai", OpenAIVariant: "openai" },
        },
    }),
)
```

`Metadata` 可选,缺省时继承 legacy 顶层模型设置。请求只带别名;base_url、密钥、headers 都留在服务端。

---

## 7. 框架层(llmagent)的通用多模型能力

openclaw 的 catalog 底层用的是框架新能力(`docs/mkdocs/zh/model.md:930`):

```go
agent := llmagent.New("my-agent",
    llmagent.WithModels(map[string]model.Model{
        "smart":  openai.New("gpt-4o"),
        "fast":   openai.New("gpt-4o-mini"),
        "vision": openai.New("gpt-4o"),
    }),
    llmagent.WithModel(openai.New("gpt-4o-mini")), // 默认模型
)

runner := runner.NewRunner("app", agent)

// 这次请求用 "smart"
eventChan, err := runner.Run(ctx, userID, sessionID, message,
    agent.WithModelName("smart"),
)
// 下一次请求仍用默认
eventChan2, err := runner.Run(ctx, userID, sessionID, message2)
```

- **优先级**:`RunOptions.Model` > `RunOptions.ModelName` > Agent 默认模型;`ModelName` 不存在则回退默认(model.md:990)
- **按模型覆盖提示词**:`llmagent.WithModelInstructions` / `WithModelGlobalInstructions` 按 `model.Info().Name` 覆盖提示词(model.md:1035)
- **`__default__` 是框架保留名**(model.md:975)
- **切换不丢会话**:切换模型不会清除会话历史(model.md:1022)

---

## 8. 怎么接第三方模型:编译期注册 + 运行期选择

设计意图写在两处注释里:`registry/registry.go:11-24`(下游仓库用匿名导入注入能力)、`app/app.go:14-16`(为分发而设计的 app 包)。

```go
// 你的企业分发 main 包里 —— 匿名导入插件
import (
    _ "openclaw"                          // 官方 app 包(注册 mock+openai)
    _ "your/module/openclaw_plugins/anthropic_model"   // ← 你的插件
)

// 插件包 anthropic_model 内部:
package anthropic_model

func init() {
    registry.RegisterModel("anthropic", func(spec registry.ModelSpec) (model.Model, error) {
        var cfg anthroConfig
        registry.DecodeStrict(spec.Config, &cfg)   // 读 model.config: 自定义字段
        return anthropic.New(spec.Name, anthropic.WithAPIKey(spec.APIKey), ...), nil
    })
}
```

用户侧启用只需改 yaml:

```yaml
model:
  mode: "anthropic"        # ← 运行期按名字选
  name: "claude-opus-5"
  config:
    max_tokens: 8192       # ← 逃生舱,你的工厂自己解码
```

> 1.11 下,第三方注册的模型同样可以进 catalog:`model.models.<alias>` 的 `mode` 指向你注册的 typeName 即可,比如 `mode: anthropic`。

---

## 9. 关键设计点提炼

| 点 | 说明 |
|---|---|
| **编译期注册 + 运行期选择** | 注册在 `init()` 完成(改代码才加新家);`model.mode` / catalog 别名运行期按名字挑。**不是动态加载器,是插件接缝** |
| **匿名导入模式** | 第三方不 fork 官方 app,在自己的 main 里 `_` 导入插件包即可 |
| **查重即冲突保护** | 同名 typeName 二次注册直接报错,避免静默覆盖 |
| **API key 走环境变量** | `modelConfig` 没有 `api_key` 字段;openai 读 `OPENAI_API_KEY`,变体读各自 env。1.11 catalog 每 entry 默认 `OpenAIUseVariantAPIKey` |
| **`config:` 是逃生舱** | 官方字段管不住的自定义配置全塞 `spec.Config`(yaml.Node),插件自己严格解码 |
| **catalog 拒绝静默回退** | 未知模型别名 HTTP 400(`invalid_model`),多租户网关下安全 |
| **内省命令** | `openclaw inspect plugins` 列出所有已注册 type(inspect_cmd.go:72) |
| **并发安全** | 全局 RWMutex,运行期 Lookup 无锁争用问题;catalog 只读快照 |

---

## 10. 局限(别被"可插拔"误导)

1. **加一家模型 = 改代码重新编译**。没有任何配置项能把一个没 import 的包注册进去。
2. `OpenAIVariant` 是 openai 专属字段;`openai_variant: auto` 的自动推断只在 openai 模式生效。
3. 7 类注册表共用一套 `validateType`/`normalizeType`,typeName 全局小写化——名字是全局唯一标识,起名要全局不冲突。
4. Model Catalog 仅 `agent-type: llm` 支持;claude-code 模式无法用。
5. catalog 选择只作用于当前主 agent run,摘要/记忆/标题/子 agent 用的是各自默认。

---

## 11. 对 EDITH 的启示

EDITH 当前只有一种模型路径。这份机制值得借鉴/直接使用的点:

### 11.1 直接能用(框架层)

- **`llmagent.WithModels(map[string]model.Model)` + `agent.WithModelName("别名")`** 每次 Run 选模型——如果 EDITH 想按任务复杂度选模型(简单问答用便宜模型、大重构用强模型),框架原生支持,不需要自己造。正好呼应 EDITH 铁律:**"每次 Run 都是定制化的:配置 + 上下文;RunOptions 是灵魂"**。
- **`WithModelInstructions` / `WithModelGlobalInstructions`** 按模型覆盖提示词——EDITH 以后接多家时,不同模型用不同 system prompt。
- **接口只 2 个方法**(`GenerateContent` + `Info`)——接模型的成本被压到最低,这是整个设计的灵魂。

### 11.2 参考但不照搬

- **openclaw 的 catalog(别名 + 拒绝未知 + 不静默回退)** 对多租户服务端必要;EDITH 是本地单用户,大概率用不到那层,直接用框架的 `WithModels` + `WithModelName` 就够。
- **配置收拢成一个 spec + 一个逃生舱 yaml.Node**——官方字段管通用,自定义字段走 `config:`,框架不需要预知每家模型的配置。EDITH 可沿用这个约定。
- **编译期注册对单机单用户产品反而是优点**——EDITH 是本地工具,不需要热加载;编译期查重比动态注册更安全。
- 不需要的部分:**7 类并排注册表**对 EDITH 过重,EDITH 只接模型的话,一个 `ModelFactory` map 就够了。

### 11.3 接入示意(EDITH 的 run.go)

```go
// internal/agent/run.go —— 伪代码示意
mdl := openai.New("gpt-5",
    openai.WithVariant(openai.VariantDeepSeek),   // 按 provider 选 variant
    openai.WithBaseURL(os.Getenv("DEEPSEEK_BASE_URL")),
)
ag := llmagent.New("edith-coder",
    llmagent.WithModels(map[string]model.Model{
        "fast":   cheapModel,
        "strong": strongModel,
    }),
    llmagent.WithModel(defaultModel),
)
// 每次 Run 决定用哪个
runOpts := []agent.RunOption{ agent.WithModelName(modelName) }  // modelName 由前端/策略决定
eventChan, err := runner.Run(ctx, uid, sid, msg, runOpts...)
```

---

## 12. 证据索引

| 事实 | 位置 |
|---|---|
| 注册表定义(maps + RWMutex) | `registry/registry.go:214-224` |
| 注册/查重/查询 API | `registry/registry.go:433-458` |
| ModelSpec / ModelFactory 契约 | `registry/registry.go:196-212` |
| 包设计意图(匿名导入) | `registry/registry.go:11-24`、`app/app.go:14-16` |
| 内置注册(mock + openai) | `app/builtins.go:16-17` |
| 单模型创建入口(消费端) | `app/app.go:3898-3944` |
| openai 工厂(拼 Option) | `app/app.go:3840-3896` |
| variant 合法值与默认 | `app/app.go:522`、`run_options.go:660-665` |
| 变体自动推断(按 host) | `app/app.go:4172-4195` |
| GLM 专用 run option | `app/app.go:4121-4135` |
| `model:` yaml 结构(1.10) | `run_options.go:1307-1317` |
| `model:` yaml 结构(1.11,含 models/default) | `run_options.go`(modelConfig 新增 `Default`、`Models`) |
| yaml → opts 映射 | `run_options.go:1924-1975` |
| model.Model 接口 | `model/model.go`(2 方法) |
| 7 个 Variant 枚举 | `model/openai/openai.go:78-96` |
| 变体配置表(每家差异) | `model/openai/openai.go:271-398` |
| 内省命令 | `app/inspect_cmd.go:62-87` |
| 各家默认 base_url | `model/openai/openai.go:51-71` |
| **Model Catalog 核心结构** | `app/model_catalog.go:27-32` |
| **ModelMetadata / ModelCatalog** | `app/runtime_options.go:85-98` |
| **yaml entry 配置** | `app/run_options.go`(modelEntryConfig) |
| **目录解析(三来源)** | `app/model_catalog.go:51-189` |
| **entry → runOptions** | `app/model_catalog.go:293-353` |
| **选择逻辑(请求→profile→默认)** | `app/model_catalog.go:368-381` |
| **gateway 集成(SelectableModels + Resolver)** | `app/model_catalog.go:436-468` |
| **catalog 版 call budget** | `app/model_catalog.go:470-504` |
| **每模型 compatibility run option** | `app/app.go:4319-4350` |
| **catalog 参与进程指纹** | `app/app.go:2685-2712` |
| 框架层 WithModels + WithModelName | `docs/mkdocs/zh/model.md:930-1036` |
| openclaw catalog 文档与 yaml 示例 | `openclaw/README.md:837-925` |
| Requesty 是 example 非 variant | `examples/model/requesty/main.go` |
