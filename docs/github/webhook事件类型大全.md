# GitHub App Webhook 事件类型大全

> GitHub App 可订阅数十种事件类型，覆盖仓库操作的方方面面。
> 每个事件包含多种 action（比如 `issues` 事件有 `opened`、`edited`、`closed` 等）。

---

## 一、代码相关

| 事件 | 触发时机 | 我们的 Agent 能做什么 |
|---|---|---|
| **push** | 代码推送到分支 | 自动触发 CI 分析、代码审查 |
| **create** | 创建了分支或 tag | 监听分支创建 |
| **delete** | 删除了分支或 tag | 清理资源 |

## 二、Issues

| 事件 | 触发时机 | 我们的 Agent 能做什么 |
|---|---|---|
| **issues** | Issue 被创建、编辑、关闭、重新打开、指派等 | ✅ **已接入** — 自动分析新 Issue |
| **issue_comment** | Issue 上的评论被创建、编辑、删除 | ✅ **已接入** — @EDITH 触发分析 |

## 三、Pull Request

| 事件 | 触发时机 | 我们的 Agent 能做什么 |
|---|---|---|
| **pull_request** | PR 被创建、同步、关闭、合并等 | 自动 review 代码、检测冲突、运行检查 |
| **pull_request_review** | PR 被 review（提交意见） | 根据 review 意见自动修改 |
| **pull_request_review_comment** | PR 代码行上被评论 | 根据行级评论修复对应代码 |
| **pull_request_review_requested** | PR 被请求 review | 自动执行 review 任务 |

## 四、Meta — Webhook 自身生命周期

| 事件 | 触发时机 |
|---|---|
| **ping** | App 刚被安装、Webhook 刚配置（测试连通性） |
| **installation** | App 被安装或卸载到仓库 |
| **installation_repositories** | App 被添加或移除仓库权限 |

## 五、项目与协作

| 事件 | 触发时机 | 我们的 Agent 能做什么 |
|---|---|---|
| **pull_request** | 见上 | 代码审查 + 自动修复 |
| **pull_request_review** | 见上 | 按 review 意见改代码 |
| **check_run** | CI 检查开始/完成 | 分析 CI 失败原因 |
| **check_suite** | 检查套件创建/完成 | 同上 |
| **status** | Commit 状态变更 | 监听构建状态 |

## 六、安全性

| 事件 | 触发时机 | 我们的 Agent 能做什么 |
|---|---|---|
| **secret_scanning_alert** | 检测到密钥泄露 | 自动通知、建议修复 |
| **dependabot_alert** | 依赖漏洞被发现 | 分析漏洞并自动升级依赖 |
| **code_scanning_alert** | Code scanning 发现告警 | 分析告警并修复 |
| **security_advisory** | GitHub 发布安全公告 | 检查项目是否受影响 |

## 七、其他

| 事件 | 触发时机 |
|---|---|
| **star** | 仓库被收藏 |
| **fork** | 仓库被 fork |
| **member** | 仓库被添加协作者 |
| **label** | 标签被创建/编辑/删除 |
| **milestone** | 里程碑被创建/编辑/关闭 |
| **release** | Release 发布 |

---

## 心智模型：三类事件

```
GitHub 事件
    │
    ├── 💻 代码事件     → push, pull_request, create, delete
    │                     Agent：审查、CI、自动修复
    │
    ├── 💬 协作事件     → issues, issue_comment, pull_request_review
    │                     Agent：分析、回复、跟进（我们在这里）
    │
    └── 🛡️ 安全事件     → secret_scanning, dependabot, code_scanning
                          Agent：自动检测、修复漏洞
```

## 我们的 MVP 聚焦

| 阶段 | 事件 | 功能 |
|---|---|---|
| ✅ 当前 | issues (opened) | 自动分析新 Issue |
| ✅ 当前 | issue_comment (created) | @EDITH 触发分析 |
| 📌 下一步 | pull_request (opened/synchronize) | 自动 review + 修复代码 |
| 🔮 未来 | pull_request_review (submitted) | 按 review 意见修改 |
| 🔮 未来 | push | 提交后自动检测 |

**Note**：所有事件都是可选的——GitHub App 配置里勾选你要订阅的就行，垃圾事件不会发过来。
