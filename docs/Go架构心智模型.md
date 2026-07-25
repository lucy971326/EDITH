# Go 架构心智模型

设计一个模块前，先不急着拆文件或写 `NewXxx`。先问：**这个长期存在的对象，应该拥有什么能力、状态与身份？**

```text
结构体字段
├── 长期能力：Runner、Sandbox、数据库、HTTP Client
├── 长期状态：缓存、Map、Mutex、配置
└── 稳定身份：App ID、基础 Prompt、目录路径

函数参数
└── 本次调用才有的输入：ctx、userID、sessionID、消息、请求选项
```

例如：

```go
type AgentRuntime struct {
	runner  runner.Runner
	sandbox sandbox.Provider
	skills  *skills.Store
}

func (r *AgentRuntime) Run(
	ctx context.Context,
	userID string,
	sessionID string,
	message model.Message,
) {
	// ...
}
```

读法：

```text
AgentRuntime
├── 有 Runner   → 能运行 Agent
├── 有 Sandbox  → 能取得工作区
└── 有 Skills   → 能读取用户 Skills

因此它负责：组织一次 Agent 运行。
它不负责：HTTP 协议、Telegram Webhook、前端渲染。
```

## 判断规则

1. 反复使用、对象需要长期拥有的东西，放结构体字段。
2. 只属于当前请求的数据，作为方法参数传入。
3. 字段不一定是接口：接口只用于确实有多种可替换实现的能力。
4. 一个结构体只承担其字段能力自然推导出的职责；不要让它跨层做无关工作。

```text
SandboxProvider
├── LocalProvider
└── E2BProvider
```

这里抽接口合理，因为 Local 和 E2B 都是同一种“提供工作区”的不同实现。

## 设计顺序

```text
先写职责
  ↓
列出该对象长期需要的能力 / 状态
  ↓
这些内容成为结构体字段
  ↓
列出一次调用的临时输入
  ↓
这些内容成为方法参数
  ↓
最后才决定文件和 package 怎么拆
```

一句话：

> Go 架构的核心，是先决定每个长期存在的对象拥有什么能力和状态，再让它只承担这些能力自然推导出的职责。
