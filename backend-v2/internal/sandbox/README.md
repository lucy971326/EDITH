# sandbox

按用户和会话隔离的 E2B 工作区，以及文件、命令、进程工具。
无 HTTP 入口；公开能力是 Module.Tools，交给 tools 模块聚合给 ManagedRunner。

## 总图

```text
main
 ├─ volume.New(DB) → volumeModule.Volumes
 └─ sandbox.New(Dependencies)
    ├─ DB + Template + Volumes
    ├─ createSchema → user_sandboxes
    ├─ E2B Client
    ├─ service{db, client, template, volumes} 私有业务能力
    └─ Module{Tools: toolSet{service}}     唯一公开能力
                                      │
                                      ▼
                              tools 模块 → ManagedRunner
                                      │
                 每次 Tool(ctx, input) │
                                      ▼
              Invocation.Session.UserID + Session.ID
                                      │
                                      ▼
                       toolSet.currentWorkspace
                         ├─ Invocation 已缓存 → 复用
                         └─ service.Workspace(user, session)
                              ├─ 有映射 → E2B Connect
                              └─ 无映射
                                   ├─ volumes.MountForUser(user)
                                   ├─ E2B Create + 挂载 skills/custom
                                   └─ 保存 user_sandboxes
                                      │
                        ┌─────────────┴─────────────┐
                        ▼                           ▼
                  workspace.Files             workspace.Commands
                  文件工具 6 个                 进程工具 4 个
```

## 对外结构

```text
Dependencies
├─ DB *sql.DB       创建并访问 user_sandboxes
├─ Template string  首次创建 E2B Sandbox 的模板
└─ Volumes *volume.Service 取得用户 Volume 挂载信息

Module
└─ Tools tool.ToolSet
   └─ 交给 tools 模块聚合

toolSet（私有）
└─ workspaces *service
   ├─ Name()  → sandbox
   ├─ Close() → 当前无额外动作
   └─ Tools() → 10 个 Agent 工具
```

New 只创建本地表、E2B 客户端和工具集合，不创建远端 Sandbox；首次调用工具时才创建。

## service：绑定用户会话与 E2B

```text
service（私有）
├─ db       *sql.DB
├─ client   *e2b.Client
├─ template string
└─ volumes  *volume.Service
```

```text
service.Workspace(ctx, userID, sessionID)
├─ 校验身份
├─ 查询 user_sandboxes
├─ 已有 sandbox_id
│  ├─ E2B Connect
│  ├─ 更新 updated_at
│  └─ 返回 Sandbox
└─ 没有映射
   ├─ volumes.MountForUser(userID)
   ├─ E2B Create(template, Secure, AutoResume, VolumeMounts)
   ├─ INSERT user_sandboxes
   └─ 返回 Sandbox
```

本地表只保存绑定关系；真正文件和进程在 E2B 云端。

```text
user_sandboxes
├─ user_id / session_id  主键，用户会话身份
├─ sandbox_id            E2B 资源身份
└─ created_at / updated_at
```

schema.go 的 createSchema 由 New 调用，其他模块不直接访问此表。

Volume 的用户绑定由 `volume` 模块独立管理：

```text
user_id → user_volumes → Volume.Name → /home/user/skills/custom
```

Sandbox 只使用 `volume.Service` 返回的挂载信息，不读取 `user_volumes` 表，也不保存 Volume Token。

## 身份从哪里来

```text
Tool(ctx, input)
├─ input：模型填写 path / command / PID
└─ ctx：框架 Invocation 提供
   └─ Session.UserID + Session.ID
```

currentWorkspace：

```text
toolSet.currentWorkspace(ctx)
├─ Invocation 不存在 → 返回错误
├─ Invocation State 有 Sandbox → 复用本次 Run
└─ 没有缓存
   ├─ service.Workspace(UserID, SessionID)
   ├─ invocation.SetState(...)
   └─ 返回 Sandbox
```

模型不能填写 userID 或 sessionID；身份永远来自 Runner 上下文。这是 Sandbox 隔离的关键。

## 工作区约定

```text
WorkspaceLayout
├─ Root      /home/user
├─ Uploads   uploads
├─ Work      work
└─ Artifacts artifacts

Workspace.WorkPath() → /home/user/work
Workspace.ToolGuide() → 注入工具描述的目录规则
```

workspacePath 只允许工作区相对路径，禁止绝对路径和 ..。
命令在 work/ 执行，长期进程日志在 .edith/processes/。

## 文件工具

```text
工具                       输入                    输出
sandbox_list_files         listFilesInput          listFilesOutput
sandbox_read_file          readFileInput           readFileOutput
sandbox_write_file         writeFileInput          fileOutput
sandbox_make_directory     pathInput               fileOutput
sandbox_move_path          movePathInput           fileOutput
sandbox_delete_path        pathInput               fileOutput
```

```text
listFilesInput   { Path, Depth }
readFileInput    { Path, Offset, Limit }
writeFileInput   { Path, Content }
pathInput        { Path }
movePathInput    { From, To }

listFilesOutput  { Path, Entries[] }
fileEntry        { Name, Path, Type, Size }
readFileOutput   { Path, Content, Truncated }
fileOutput       { Path, Message }
```

每个文件工具：currentWorkspace(ctx) → workspace.Files.*。

## 进程工具

```text
工具                       输入                    输出
sandbox_run_command        runCommandInput         runCommandOutput
sandbox_start_process      startProcessInput       startProcessOutput
sandbox_list_processes     struct{}                listProcessesOutput
sandbox_kill_process       processInput            killProcessOutput
```

```text
runCommandInput   { Command, Args, Envs, TimeoutMs }
startProcessInput { Command, Args }
processInput      { PID }

runCommandOutput  { ExitCode, Stdout, Stderr, StdoutTruncated, StderrTruncated }
startProcessOutput{ PID, LogPath }
processInfo       { PID, Command, Args, Cwd, Tag }
listProcessesOutput { Processes[] }
killProcessOutput  { PID, Message }
```

每个进程工具：currentWorkspace(ctx) → workspace.Commands.*。

## 一句话记忆

```text
Module          = 只暴露 Sandbox 工具集合
toolSet         = 从 Invocation 取身份并创建工具
service         = user + session → E2B Sandbox + 用户 Volume 挂载
user_sandboxes  = 保存本地绑定关系
user_volumes    = 由 Volume 模块保存用户持久化绑定
Workspace       = 统一路径和目录安全规则
文件工具       = 操作当前 Sandbox.Files
进程工具       = 操作当前 Sandbox.Commands
```
