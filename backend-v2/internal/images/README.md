# images

图片模块负责一张图片从浏览器上传、保存、校验，到 Agent 使用和会话历史展示的完整生命周期。

## 一张图看懂整个模块

```text
┌─────────────────────────────────────────────────────────────────────────────┐
│                         images 模块                                          │
│                                                                             │
│  main 创建                                                                   │
│    │                                                                        │
│    ├─ Dependencies                                                           │
│    │  ├─ DB *sql.DB ───────────────────────────────► store                  │
│    │  └─ Config ───────────────────────────────────► cosStore               │
│    │                                                                        │
│    ▼                                                                        │
│  Module                                                                      │
│    ├─ HTTP ─────────────── Web BFF                                           │
│    │  ├─ CreateUpload(req)                                                   │
│    │  │    └─ CreateUploadRequest                                             │
│    │  │         └─ service.CreateUpload                                      │
│    │  │              ├─ store.insert ───────► chat_images (pending)          │
│    │  │              └─ cos.signPut ────────► UploadURL                      │
│    │  │                                                                       │
│    │  ├─ CompleteUpload(req)                                                  │
│    │  │    └─ CompleteUploadRequest                                           │
│    │  │         └─ service.CompleteUpload                                    │
│    │  │              ├─ store.forUser                                         │
│    │  │              ├─ cos.head ───────────► 校验对象                        │
│    │  │              └─ store.markReady ────► chat_images (ready)             │
│    │  │                                                                       │
│    │  └─ OpenImage(req)                                                       │
│    │       └─ service.OpenForUser                                             │
│    │            ├─ store.forUser ───────────► 校验 user + ready               │
│    │            └─ cos.signGet ────────────► 302 临时读取地址                 │
│    │                                                                        │
│    ├─ AgentInput ─────── AgentRun                                             │
│    │  └─ AddMessageImages(user, session, imageIDs, message)                  │
│    │       ├─ service.openForSession ─────► 校验图片属于本会话               │
│    │       ├─ cos.signGet ────────────────► 给模型短期 URL                    │
│    │       └─ context 保存 URL → imageID 映射                                │
│    │                                                                        │
│    └─ SessionImages ─── AgentRun / ManagedRunner 的 Session Service           │
│       └─ Wrap(next)                                                          │
│            └─ imageSessionService                                            │
│                 ├─ AppendEvent ── runtime URL → edith-image://ID 再落库       │
│                 └─ GetSession ─── edith-image://ID → COS URL 再给前端          │
│                                                                             │
│  私有实现                                                                   │
│    service ──► store + cosClient                                             │
│    store ────► SQLite chat_images                                            │
│    cosStore ─► 腾讯云 COS                                                    │
└─────────────────────────────────────────────────────────────────────────────┘
```

## 模块对外的三项能力

```text
Module
├─ HTTP *HTTP
│  └─ 给 Web BFF：上传、确认、读取图片
├─ AgentInput *AgentInput
│  └─ 给 AgentRun：把图片 ID 加入本次模型输入
└─ SessionImages *SessionImages
   └─ 给 Runner：包装会话存储，处理图片持久标记与临时 URL
```

`Module` 只是能力集合，不承载业务逻辑。`main` 创建它，并把三个能力交给对应调用方。

## 创建模块时的依赖

```text
Dependencies
├─ DB *sql.DB
│  └─ 模块创建 chat_images 表，store 使用它读写元数据
└─ Config Config
   ├─ Bucket
   ├─ Region
   ├─ SecretID
   └─ SecretKey
      └─ 只用于创建 cosStore，不对外暴露
```

```text
Config
└─ COS 服务端连接配置

New(Dependencies)
├─ 校验 DB 和 COS 配置
├─ 创建 cosStore
├─ 创建 store
├─ 创建 service{store, cos}
├─ 创建 AgentInput / SessionImages / HTTP
└─ 返回 Module
```

## 核心数据结构扁平展开

### Image：浏览器和 Agent 共同认识的图片身份

```text
Image
├─ ID       string  图片稳定 ID，例如 img_xxx
└─ MimeType string  image/png、image/jpeg 等
```

服务：HTTP 响应、Agent 输入相关代码、前端图片展示。

它只代表图片身份，不包含 COS 密钥、对象 Key 或用户字段。

### UploadInput：业务层上传预留输入

```text
UploadInput
├─ SessionID string
├─ MimeType  string
└─ SizeBytes int64
```

服务：`service.CreateUpload`。它是内部 Go 参数，不是 HTTP JSON 契约。

### CreateUploadRequest / Response：创建上传预留的 HTTP 契约

```text
CreateUploadRequest                  CreateUploadResponse
├─ UserID    string                  ├─ Image     Image
├─ SessionID string                  └─ UploadURL string
├─ MimeType  string
└─ SizeBytes int64
```

流程：

```text
浏览器 → HTTP.CreateUpload
       → service.CreateUpload
       → 返回 Image + UploadURL
浏览器 → 使用 UploadURL 直传 COS
```

### CompleteUploadRequest / Response：确认上传完成的 HTTP 契约

```text
CompleteUploadRequest               CompleteUploadResponse
└─ UserID string                     └─ Image Image
```

`imageID` 来自 URL 路径 `/internal/images/{imageID}/complete`，不是 JSON 字段。

## HTTP 入口与方法归属

```text
HTTP
└─ service *service
   └─ HTTP 只解析请求、校验输入、调用 service、写 JSON
```

```text
HTTP.Register(mux)
├─ POST /internal/images
│  └─ CreateUpload
│     ├─ 读 CreateUploadRequest
│     ├─ 调 service.CreateUpload
│     └─ 返回 CreateUploadResponse（201）
│
├─ POST /internal/images/{imageID}/complete
│  └─ CompleteUpload
│     ├─ 读 CompleteUploadRequest
│     ├─ 调 service.CompleteUpload
│     └─ 返回 CompleteUploadResponse（200）
│
└─ GET /internal/images/{imageID}
   └─ OpenImage
      ├─ 读 query userId 和 path imageID
      ├─ 调 service.OpenForUser
      └─ 重定向到 COS 临时读取 URL（302）
```

## service：图片业务流程

```text
service
├─ store *store
│  └─ 图片元数据和归属
└─ cos cosClient
   └─ COS 签名、校验、删除
```

```text
service.CreateUpload(ctx, userID, UploadInput)
├─ 校验用户、会话、MIME 类型、大小
├─ 生成 imageID 和 objectKey
├─ store.insert(status=pending)
├─ cos.signPut(objectKey)
└─ 返回 Image + UploadURL

service.CompleteUpload(ctx, userID, imageID)
├─ store.forUser
├─ cos.head 校验对象大小和 MIME
├─ 不匹配 → cos.delete + store.delete
└─ 匹配 → store.markReady

service.OpenForUser(ctx, userID, imageID)
├─ store.forUser(readyOnly=true)
└─ cos.signGet
   └─ 返回短期读取 URL

service.openForSession(ctx, userID, sessionID, imageID)
├─ store.forSession(readyOnly=true)
└─ cos.signGet
   └─ 只允许当前用户、当前会话使用图片
```

`service` 是业务规则所在的位置；HTTP、AgentRun 和会话包装都通过它使用图片能力。

## store：数据库私有实现

```text
store
└─ db *sql.DB
```

```text
chat_images
├─ image_id
├─ user_id
├─ session_id
├─ object_key
├─ mime_type
├─ size_bytes
├─ status       pending / ready
└─ created_at
```

```text
store.createSchema       → New 时创建表
store.insert             → 保存 pending 图片预留
store.forUser            → 按用户读取图片，可要求 ready
store.forSession         → 按用户 + 会话读取 ready 图片
store.markReady          → 确认 COS 对象后改为 ready
store.delete             → 删除无效预留或对象对应的元数据
```

`store` 和 `imageRecord` 都是模块私有实现，不被其他模块直接调用。

```text
imageRecord
├─ Image                   对外图片身份
├─ userID / sessionID      归属校验
├─ objectKey               COS 对象位置
├─ sizeBytes               上传大小校验
└─ status                  pending / ready
```

## AgentInput：本次模型调用的图片输入

```text
AgentInput
└─ service *service
```

```text
AgentInput.AddMessageImages(ctx, userID, sessionID, imageIDs, message)
├─ 校验 imageIDs 不为空、无重复
├─ service.openForSession
│  └─ 校验图片属于当前用户和当前会话
├─ 把每张图片的短期 COS URL 加到 model.Message
└─ 在 context 保存：runtime URL → imageID
```

服务：`AgentRun` 组装模型请求时调用。它不处理 HTTP，也不直接查 SQLite。

```text
Reference(imageID)
└─ 返回 edith-image://imageID

ImageIDFromReference(value)
└─ 从持久标记取回 imageID

WithHydratedSession(ctx)
└─ 标记读取会话时需要把持久标记换回 COS URL
```

## SessionImages：会话历史的图片转换层

```text
SessionImages
└─ service *service
```

```text
SessionImages.Wrap(next session.Service)
└─ 返回 imageSessionService
   ├─ 保留框架原始 Session Service
   └─ 在读写事件时加入图片转换
```

```text
imageSessionService.AppendEvent
├─ 复制事件
├─ runtime COS URL → edith-image://imageID
├─ 调用原始 Session Service 落库
└─ 更新当前运行时会话

imageSessionService.GetSession
├─ 调用原始 Session Service 读历史
├─ 如果 ctx 标记 WithHydratedSession
├─ edith-image://imageID → service.openForSession → COS URL
└─ 返回给 AgentRun / 前端投影
```

这样数据库永远不保存短期签名 URL：

```text
模型输入 / 运行时：COS 临时 URL
          │
          ▼
落库前：edith-image://img_xxx
          │
          ▼
重新读取并需要展示时：COS 临时 URL
```

## COS 私有实现

```text
cosClient（小接口，只是 COS 替换边界）
├─ signPut(ctx, key) → 上传预签名 URL
├─ signGet(ctx, key) → 读取预签名 URL
├─ head(ctx, key)    → 读取对象大小和 MIME
└─ delete(ctx, key)  → 删除无效对象
```

```text
cosStore
├─ client *cos.Client
├─ secretID
└─ secretKey
```

`cosClient` 只存在于 COS 外部服务替换或测试边界；业务代码依赖这四个具体能力，不处理腾讯云 SDK 细节。

## 一句话记忆

```text
HTTP          = 接收浏览器请求
service       = 决定图片能不能上传、确认、读取
store         = 保存图片元数据和归属
cosStore      = 访问 COS 对象
AgentInput    = 把图片加入本次模型输入
SessionImages = 把临时 URL 和持久标记互相转换
Module        = 把三项公开能力交给 main 组装
```
