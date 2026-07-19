---
name: sandbox-integration-pattern
description: trpc-agent-go 沙箱集成模式——用 workspaceio 封装自定义 Backend，不走 workspace_exec。用于 E2B/容器沙箱对接时参考。
---

# 沙箱集成模式：workspaceio vs workspace_exec

## 核心结论

**不要用 workspace_exec，直接包装 workspaceio。**

## 架构对比

### ❌ 不推荐：用 workspace_exec

```
workspaceio（底层）
  ↓
workspace_exec（上层工具，只暴露 RunProgram）
  ↓
LLM 通过 shell 命令操作文件（cat/echo）
```

问题：没有结构化文件工具，LLM 生成 shell 命令容易出错。

### ✅ 推荐：包装 workspaceio

```
workspaceio（底层）
  ↓
CloudSandboxBackend（我们写的，~50行）
  ↓
FileToolSet（已有）
  ↓
Agent 只用 FileToolSet，完全不碰 workspace_exec
```

## 实现步骤

### 1. CloudSandboxBackend 实现

```go
type CloudSandboxBackend struct {
    // workspace 从 ctx 获取，不手动创建
}

func (b *CloudSandboxBackend) ReadFile(ctx context.Context, path string) (string, error) {
    ws, ok := workspaceio.WorkspaceFromContext(ctx)
    if !ok {
        return "", fmt.Errorf("no workspace in context")
    }
    files, err := ws.Collect(ctx, path)
    if err != nil {
        return "", err
    }
    return string(files[0].Data), nil
}

func (b *CloudSandboxBackend) WriteFile(ctx context.Context, path string, content string) error {
    ws, ok := workspaceio.WorkspaceFromContext(ctx)
    if !ok {
        return fmt.Errorf("no workspace in context")
    }
    return ws.PutFiles(ctx, codeexecutor.PutFile{
        Path:    path,
        Content: []byte(content),
    })
}

func (b *CloudSandboxBackend) ExecCommand(ctx context.Context, cmd string, workDir string) (string, error) {
    ws, ok := workspaceio.WorkspaceFromContext(ctx)
    if !ok {
        return "", fmt.Errorf("no workspace in context")
    }
    result, err := ws.RunProgram(ctx, codeexecutor.RunProgramSpec{
        Cmd: "bash",
        Args: []string{"-c", cmd},
        Cwd: workDir,
    })
    if err != nil {
        return "", err
    }
    return result.Stdout, nil
}
```

### 2. Agent 配置

```go
b := backend.NewCloudSandboxBackend()
ft := tools.NewFileToolSet(b)

ag := llmagent.New("github-edith",
    llmagent.WithModel(m),
    llmagent.WithToolSets([]tool.ToolSet{ft, gh}),
    // 可选：关闭 workspace_exec，只用 FileToolSet
    llmagent.WithWorkspaceExecSurfaceEnabled(false),
)
```

## workspaceio 方法速查

| 方法 | 对应 Backend 方法 | 用途 |
|------|------------------|------|
| `ws.Collect(ctx, patterns...)` | `ReadFile(ctx, path)` | 按 glob 读文件 |
| `ws.PutFiles(ctx, files...)` | `WriteFile(ctx, path, content)` | 写文件 |
| `ws.RunProgram(ctx, spec)` | `ExecCommand(ctx, cmd, workDir)` | 执行命令 |
| `ws.SaveArtifact(relPath)` | 可选 | 保存产物供下一轮使用 |

## workspace 从哪里来

**不需要手动创建**，框架自动放入 ctx：

```go
// 在 Callback 里（BeforeAgent / AfterAgent / AfterTool）
ws, ok := workspaceio.WorkspaceFromContext(ctx)
```

## 关键文档位置

| 文件 | 内容 |
|------|------|
| `codeexecutor/workspaceio/workspace_io.go` | Workspace 接口定义 |
| `codeexecutor/workspaceio/context.go` | WithWorkspace / WorkspaceFromContext |
| `examples/workspace_io/` | 官方示例 |
| `agent/llmagent/llm_agent.go` | Workspace 注入 ctx 的时机 |
