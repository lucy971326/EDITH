# Workspace I/O 示例（通俗版）

## 这个示例在讲什么

假设你在做一个 Agent，它会在 workspace（工作目录）里创建、修改文件。

**问题：Agent 跑完之后，这些文件怎么保存下来？**

答案：用 `workspaceio`，在 Agent 运行前后读写 workspace 里的文件。

---

## 一个生活类比

把 workspace 想象成**一块白板**：

```
BeforeAgent（Agent 开始前）
  → 你在白板上画了两个草稿（PutFiles）

Agent 运行中
  → Agent 可能在白板上涂涂改改

AfterAgent（Agent 结束后）
  → 你把白板上的内容拍照保存（Collect）
  → 照片存到你的相册里（directorySink.Save）
```

---

## 核心代码只有三行

```go
// 1. 拿到白板
ws, ok := workspaceio.WorkspaceFromContext(ctx)

// 2. 拍照（按 glob 读取文件）
files, err := ws.Collect(ctx, "skills/*/SKILL.md")

// 3. 保存到本地目录
for _, f := range files {
    sink.Save(ctx, args.Invocation, f)
}
```

**就这么简单，没了。**

---

## workspaceio 能干什么

| 方法 | 白板类比 | 实际用途 |
|------|---------|---------|
| `ws.PutFiles()` | 往白板上画画 | 往 workspace 写文件 |
| `ws.Collect()` | 拍照保存白板内容 | 从 workspace 读文件（支持 glob） |
| `ws.RunProgram()` | 在白板旁边跑个计算器 | 在 workspace 里执行命令 |
| `ws.SaveArtifact()` | 把白板内容裱起来展览 | 保存文件供下一轮 Agent 使用 |

---

## workspace 从哪里来

**你不需要自己创建**，框架自动帮你放到 ctx 里：

```go
// 只需要这一行，就能拿到 workspace
ws, ok := workspaceio.WorkspaceFromContext(ctx)

// 如果没有配置 code executor，ok 会是 false
if !ok {
    return nil, nil  // 没有 workspace，直接返回
}
```

---

## 完整流程图

```
BeforeAgent 回调
  │
  ├── ws.PutFiles() 往 workspace 写文件（草稿）
  │
  ▼
Agent 运行
  │
  ├── Agent 可以读写 workspace（用 workspace_exec 或我们自己的工具）
  │
  ▼
AfterAgent 回调
  │
  ├── ws.Collect() 按 glob 读取 workspace 文件
  │
  ├── 遍历每个文件，保存到本地目录
  │
  └── 打印保存结果
```

---

## 和 EDITH 的关系

我们现在做的 CloudSandboxBackend，就是用同样的思路：

| 示例做法 | EDITH 做法 |
|---------|-----------|
| BeforeAgent 里 PutFiles | Agent 调用 WriteFile 工具 |
| AfterAgent 里 Collect | Agent 调用 ReadFile 工具 |
| directorySink 保存文件 | FileToolSet 读写文件 |

**底层能力完全一样，只是封装形式不同。**

---

## 几个需要注意的点

### 1. 失败时跳过

如果 Agent 运行出错，workspace 里的文件可能不可靠，直接跳过保存：

```go
if args.Error != nil {
    return nil, nil  // 出错了，不保存
}
```

### 2. 文件可能被截断

大文件（超过几 MiB）会被截断，`f.Truncated` 会是 `true`：

```go
if f.Truncated {
    log.Printf("警告：%s 被截断了，只有部分内容", f.Path)
}
```

### 3. glob 模式要精确

`skills/**` 可能匹配很多文件，注意容量控制：

```go
// ✅ 好：精确匹配
files, _ := ws.Collect(ctx, "skills/*/SKILL.md")

// ⚠️ 小心：可能匹配太多文件
files, _ := ws.Collect(ctx, "**/*")
```

### 4. 非零退出码不是 error

命令执行失败（退出码非零）通过 `RunResult.ExitCode` 返回，不是 Go error：

```go
result, _ := ws.RunProgram(ctx, spec)
if result.ExitCode != 0 {
    log.Printf("命令失败，退出码：%d", result.ExitCode)
}
```

---

## 运行方法

```bash
# 设置 API Key
export OPENAI_API_KEY="你的key"

# 可选：指向 DeepSeek 等兼容接口
export OPENAI_BASE_URL="https://api.deepseek.com/v1"

# 运行示例
cd examples/workspace_io
go run . -model deepseek-v4-flash -store ./skills_store
```

---

## 一句话总结

**workspaceio = 给你一个白板，你可以在 Agent 运行前后读写上面的内容。**
