# volume

Volume 模块负责用户级 E2B 持久化资源，不负责 Sandbox 会话，也不注册 HTTP。

```text
main.go
  ├─ volume.New()
  │    └─ Module
  │         └─ Volumes *Service
  │
  └─ sandbox.New({Volumes: volumeModule.Volumes})
       └─ Sandbox 创建时挂载用户 Volume
```

## 核心映射

```text
Clerk user_id
      │
      ▼
user_volumes（SQLite）
      │
      ├─ volume_id
      ├─ volume_name
      └─ volume_token（仅后端保存）
```

每个用户只有一个 Volume。首次调用 `MountForUser` 时创建 E2B Volume，之后直接复用数据库中的映射。首次创建由 Service 串行保护，单实例下不会为同一用户创建多个远端 Volume。

## Sandbox 挂载

```text
Volume.Name
    │
    ▼
/home/user/skills/custom
```

Sandbox 模块继续负责：

```text
user_id + session_id → sandbox_id
```

它只调用 `Volumes.MountForUser(userID)`，不读取 `user_volumes` 表，也不负责 Volume 的创建和 Token 管理。

## 对外能力

```go
MountForUser(ctx, userID) (Mount, error)
```

供 Sandbox 创建时取得挂载信息。

```go
OpenForUser(ctx, userID) (*e2bvolume.Volume, error)
```

供未来 Skills 模块读取或修改用户 Volume 文件。Volume 本身暂时没有 HTTP；前端需要展示 Skills 时，由 Skills HTTP 调用这个 Service。
