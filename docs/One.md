# GitHub Agent - 架构心智模型

## 一句话理解

> 用户在 Issue 里 @githubapp，服务端 Agent 自动拉代码、修 bug、提 PR。

## 心智模型

```
┌──────────────────────────────────────────────────┐
│                   你的大脑                         │
│                                                  │
│  看到 Issue → 理解问题 → 找到代码 → 改代码 → 提 PR │
└──────────────────────────────────────────────────┘
                     ↑ 我们要让机器做的事
                     ↓ 拆成三层

┌────────────┐    ┌────────────┐    ┌────────────┐
│  GitHub    │    │ API Server │    │   Agent    │
│  App       │───→│  耳朵+眼睛  │───→│   大脑     │───→ 回来
│  触发入口   │    │  过滤+认证   │    │  思考+执行  │
└────────────┘    └────────────┘    └────────────┘
   "通知来了"      "是谁？合法吗？"    "我来干活"
```

## 三层心智

| 层 | 角色 | 类比 | 做什么 |
|---|---|---|---|
| **GitHub App** | 门铃 | 收到快递按门铃 | 监听事件，推送通知给服务端 |
| **API Server** | 门卫 | 查身份证、过滤广告 | 接收请求、验签、过滤无效触发 |
| **Agent Worker** | 工匠 | 拿到图纸开始干活 | Clone 代码 → LLM 分析 → 修复 → 提 PR |

## 完整流程

```mermaid
sequenceDiagram
    actor User as 用户
    participant GH as GitHub App
    participant API as API Server
    participant Agent as Agent Worker
    participant LLM as LLM (Claude/GPT)
    participant Repo as Git 仓库

    User->>GH: 在 Issue 评论 "@githubapp fix this"
    GH->>API: Webhook POST（Issue Comment 事件）
    API->>API: 验证签名 ✅ 过滤无关注释 ✅
    API->>Agent: 传递任务（repo, issue, 指令）

    Agent->>Repo: Clone 仓库
    Agent->>Agent: 读取 Issue 标题+描述

    loop 思考与修复
        Agent->>LLM: 发送上下文（Issue + 代码）
        LLM->>Agent: 返回修复方案
        Agent->>Repo: 应用代码修改
    end

    Agent->>Repo: 创建分支 → Commit → Push
    Agent->>GH: 创建 PR（关联 Issue）
    Agent->>GH: 回复 Issue 评论

    GH->>User: 通知："已修复，见 PR #xxx"
```

## 认证体系 — 这些凭据都是干嘛的

### 凭据一览

| 凭据 | 是什么 | 为什么需要 |
|---|---|---|
| **App ID** | GitHub 给你 App 发的身份证号 | 告诉 GitHub "我是谁" |
| **Private Key** | App 的私钥，只有你有 | 用 RSA 签名证明"我真的是这个 App" |
| **Webhook Secret** | 你和 GitHub 之间的暗号 | 防止别人伪造请求骗你 |

### 两个完全不同的认证场景

```mermaid
flowchart TB
    subgraph 场景一["场景一：GitHub → 你的服务器（Webhook 推送）"]
        direction LR
        GH1[GitHub] -->|"POST + 签名(用 Secret 加密)"| Server[你的服务器]
        Server -->|"用同样的 Secret 解密验证"| Check1{签名匹配？}
        Check1 -->|✅ 通过| Handle[处理事件]
        Check1 -->|❌ 不匹配| Reject[拒绝，伪造请求]
    end

    subgraph 场景二["场景二：你的服务器 → GitHub（调 API 回复/提 PR）"]
        direction LR
        App["你的 App"] -->|"用 Private Key 签名 JWT"| GitHubAPI[GitHub API]
        GitHubAPI -->|"验证 JWT 签名"| Check2{是合法 App？}
        Check2 -->|✅ 通过| Token[返回临时 Token]
        Token -->|"用 Token 操作"| Op[回复 Issue / 提 PR]
    end

    style 场景一 fill:#1a1a2e,stroke:#e94560,color:#fff
    style 场景二 fill:#1a1a2e,stroke:#0f3460,color:#fff
```

### 一句话总结

- **Webhook Secret**：别人推消息给你时，你验证真假用的（防伪造）
- **Private Key**：你去 GitHub 干活时，证明你是你用的（身份认证）

### 时序：两个认证分别在哪一步

```mermaid
sequenceDiagram
    actor User as 用户
    participant GH as GitHub
    participant Server as 你的服务器
    participant API as GitHub API

    User->>GH: 评论 "@githubapp fix"
    GH->>Server: Webhook（附带 Secret 签名）
    Note over Server: 🔑 用 Webhook Secret 验签
    Server->>Server: 验签通过 ✅

    Server->>Server: Agent 开始干活...
    Server->>API: 创建 PR（附带 JWT 签名）
    Note over API: 🔑 用 Public Key 验证 JWT
    API->>Server: 返回临时 Token ✅
    Server->>API: 用 Token 回复 Issue
    API->>User: 通知："已修复，见 PR #xxx"
```

## 项目结构

```
GithubAgent/
├── main.go              # 启动入口
├── config/              # 配置管理
├── server/              # API Server（Gin）
│   └── webhook.go       # 接收 GitHub Webhook
├── github/              # GitHub 操作
│   ├── auth.go          # JWT + Token 认证
│   └── git.go           # Clone/Push/PR
├── agent/               # Agent Worker（trpc-agent-go）
│   ├── agent.go         # 核心编排
│   ├── tools.go         # 工具：读写文件、搜索、执行命令
│   └── prompt.go        # System Prompt
└── model/               # LLM Provider 接口
    └── provider.go      # 可插拔 LLM 支持
```
