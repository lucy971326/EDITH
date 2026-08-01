# cronjob

定时任务模块负责任务定义、HTTP 管理、到点调度和 Agent 创建任务。
执行任务由 cronadapter 完成；cronjob 只通过 JobRunner 调用它。

## 一张图看懂两条链路

```text
Web BFF ──HTTP CRUD──► HTTP ──► store ──► cron_jobs

main ──► Scheduler.Run(ctx)
          ├─ initializeNextRuns
          └─ 每 10 秒
             ├─ dueJobs：找 next_run_at <= now
             ├─ claimDue：原子 running = 1
             └─ go JobRunner.RunJob(job) ─► cronadapter
                                               │
                                               ▼
                                      Gateway → AgentRun
                                               │
                                      RunJob 返回
                                               ▼
                             finishRun：running = 0
                              ├─ once → disabled
                              └─ recurring → 下次时间

Agent Tool ──Invocation.Session.UserID──► store.Create
```

Scheduler 不认识 Gateway，只认识 JobRunner。调度器停止只阻止新任务，已抢占任务继续收尾。

## 对外结构

```text
Dependencies（main 传入）
├─ DB       *sql.DB
└─ Settings *userconfig.Settings

Module
├─ Tools tool.ToolSet ─► tools 模块 ─► ManagedRunner
├─ HTTP  *HTTP         ─► Web CRUD
└─ store *store        私有，Tools / HTTP / Scheduler 共用

Module.NewScheduler(JobRunner) → Scheduler
```

Module 只公开 Tools、HTTP 和 NewScheduler，数据库 store 不被其他模块直接访问。

## Job：任务定义

```text
Job
├─ ID / ClerkUserID    身份
├─ Name / Prompt       任务名称和 Agent 指令
├─ TaskType            once / recurring
├─ Schedule            RFC3339 或五段 cron
├─ Enabled / Running   用户开关 / 调度占用
├─ NextRunAt           下一次触发时间
└─ CreatedAt           创建时间
```

任务结果不写 cron_jobs；cronadapter 使用 cron:jobID 会话，结果进入会话历史。

## 输入输出结构

```text
JobInput（内部 store 输入）  { Name, TaskType, Schedule, Prompt }

CreateRequest（HTTP 输入）
{ UserID, Name, TaskType, Schedule, Prompt, Timezone }
  └─ UserID 由 BFF 注入；Timezone 创建时可保存到 userconfig

UpdateRequest = CreateRequest

JobResponse（HTTP 输出）
{ ID, Name, TaskType, Schedule, Prompt,
  Enabled, Running, NextRunAt, CreatedAt }

ListResponse
└─ Jobs []JobResponse

Agent Tool
├─ createJobInput  { Name, TaskType, Schedule, Prompt }
└─ createJobOutput { ID, Name, TaskType, Schedule, NextRunAt, Message }
```

HTTP 契约和 Agent Tool 契约分开；模型永远不填写用户 ID。

## store：CRUD、抢占和收尾

```text
store（私有）
├─ db       *sql.DB
└─ settings *userconfig.Settings
```

```text
Create(userID, JobInput)       → 校验、算首次时间、INSERT
List(userID)                   → 用户任务列表
Update(userID, jobID, input)   → 校验、重算 next_run_at、更新归属任务
Delete(userID, jobID)          → 删除归属任务
SetEnabled(userID, jobID, on)  → 切换开关，必要时重算时间

dueJobs(now)                   → enabled=1,running=0,到点任务
claimDue(jobID, now)           → 原子 running=1，RowsAffected=1 才成功
finishRun(jobID, now)          → running=0，并安排下一次
initializeNextRuns(now)        → 启动时补齐空 next_run_at
```

claimDue 是单实例防重复执行的关键；不依赖分布式锁。

## cron_jobs：配置与占用状态

字段：id、clerk_user_id、name、task_type、schedule、prompt、enabled、next_run_at、running、created_at。
schema.go 的 createSchema 由 New 调用；表只属于 cronjob 模块。

## Scheduler：后台守护循环

```text
Scheduler
├─ jobs     *store
├─ runner   JobRunner
└─ interval 10s

Run(ctx)
├─ initializeNextRuns
├─ ticker → scheduleDueJobs
│  ├─ dueJobs
│  ├─ claimDue
│  └─ go runClaimedJob
└─ ctx.Done → 停止扫描

runClaimedJob
├─ context.Background 执行已抢占任务
├─ runner.RunJob(job)
└─ jobs.finishRun(job.ID)（无论成功失败）
```

JobRunner 是调度器与执行渠道之间的唯一替换边界，实际实现是 cronadapter。

## 时间规则

```text
once
└─ Schedule = RFC3339 触发时间

recurring
├─ Schedule = 五段 cron 表达式
├─ Settings.LoadTimezone(userID)
├─ 空时区 → Asia/Shanghai
└─ robfig/cron 计算下一次时间
```

schedule_rules.go 只负责校验和时间计算，不执行任务。

## HTTP：任务管理入口

```text
HTTP
├─ jobs     *store
└─ settings *userconfig.Settings

Register(mux)
├─ GET    /internal/cron-jobs
├─ POST   /internal/cron-jobs
├─ PUT    /internal/cron-jobs/{jobID}
├─ DELETE /internal/cron-jobs/{jobID}
└─ POST   /internal/cron-jobs/{jobID}/toggle?enabled=true|false
```

HTTP 只解析契约、调用 store、返回 JobResponse；不参与调度。

## Tools：Agent 创建任务

```text
toolSet
└─ jobs *store
   ├─ Name()  → cronjob
   ├─ Close() → 当前无额外动作
   └─ Tools() → create_cron_job

create_cron_job
├─ 模型填写 createJobInput
├─ Invocation.Session.UserID 提供身份
├─ store.Create(userID, JobInput)
└─ 返回 createJobOutput
```

## 一句话记忆

```text
HTTP      = 管理任务定义
Tool      = 创建当前用户任务
Store     = CRUD + 原子抢占 + 执行收尾
Scheduler = 到点发现并启动任务
JobRunner = 调度器到 cronadapter 的边界
cron_jobs = 任务配置和 running 状态
```
