# go-github 帮我们覆盖的工作

> 项目地址：<https://github.com/google/go-github>

---

## 一、Webhook 接收 — 再也不用手写 JSON 结构体

### 之前我们手动做的
```go
// 每加一个新事件就要写一个结构体，麻烦😅
type webhookPayload struct {
    Action     string `json:"action"`
    Issue      struct { ... } `json:"issue"`
    Comment    struct { ... } `json:"comment"`
    Repository struct { ... } `json:"repository"`
}
```

### go-github 帮我们做了
- `github.ValidatePayload(r, secret)` — **签名验证**，一行搞定
- `github.ParseWebHook(eventType, payload)` — 自动识别事件类型并解析成强类型结构体
- 几十种事件类型全内置，加事件 = 加一行 `case`

### 覆盖的事件类型（我们可能用到的）

| 事件 | Go 结构体 | 用途 |
|---|---|---|
| `issues` | `IssuesEvent` | Issue 创建、关闭、重新打开等 |
| `issue_comment` | `IssueCommentEvent` | Issue 评论 |
| `pull_request` | `PullRequestEvent` | PR 创建、同步等 |
| `push` | `PushEvent` | 代码推送 |

---

## 二、GitHub API 客户端 — 操作仓库

### 我们未来需要用到的 API 操作

| 操作 | 代码示例 | 阶段 |
|---|---|---|
| 创建 Issue 评论 | `client.Issues.CreateComment(ctx, owner, repo, number, comment)` | ✅ 即将使用 |
| 获取 Issue 详情 | `client.Issues.Get(ctx, owner, repo, number)` | 📌 后续 |
| 创建 PR | `client.PullRequests.Create(ctx, owner, repo, pr)` | 📌 后续 |
| 获取文件内容 | `client.Repositories.GetContents(ctx, owner, repo, path, opts)` | 📌 后续 |
| 创建/更新文件 | `client.Repositories.CreateFile(ctx, owner, repo, path, opts)` | 📌 后续 |

### 认证处理
```go
// 用 JWT 生成的 Installation Token 创建客户端
client := github.NewClient(nil).WithAuthToken(installationToken)

// 然后就随便用了...
client.Issues.CreateComment(ctx, owner, repo, number, comment)
```

---

## 三、对比：接入前后变化

| 工作 | 之前（手写） | 之后（go-github） |
|---|---|---|
| 解析 webhook payload | 手写 struct + json.Decode | `ParseWebHook()` 全自动 |
| 签名验证 | ❌ 没做 | `ValidatePayload()` 一行 |
| 判断事件类型 | 自己读 header | 传入 event type 即可 |
| Issue 评论 | 要写 POST 请求 | `client.Issues.CreateComment()` |
| PR 创建 | 要写 POST 请求 | `client.PullRequests.Create()` |
| 文件操作 | 要写 HTTP 请求 | `client.Repositories.*` |

---

## 四、仍需自己做的

| 工作 | 说明 |
|---|---|
| **JWT 生成** | go-github 不提供，用 `golang-jwt/jwt` 生成（已有 `test/test_install.py` 验证过流程） |
| **Agent 核心逻辑** | 我们的业务核心，`trpc-agent-go` 编排 |
| **Git 操作（clone/push）** | 用 `go-git` 或命令行 git |
| **@提及检测** | 我们的业务逻辑：判断是否 mention 了我们的 bot |
| **ngrok 内网穿透** | 开发调试工具 |

---

## 五、接入后架构

```
GitHub 发 Webhook
    │
    ▼
go-github.ValidatePayload()     ← 签名验证（安全）
    │
    ▼
go-github.ParseWebHook()        ← 解析成强类型
    │
    ▼
switch event.(type)
    ├── *github.IssuesEvent       → 处理 Issue 打开
    ├── *github.IssueCommentEvent → 处理 @提及
    └── ...（后续扩展）
    │
    ▼
我们的 agent.AnalyzeIssue()     ← 业务逻辑（不变）
    │
    ▼
go-github Client                ← 回复 Issue、创建 PR
```
