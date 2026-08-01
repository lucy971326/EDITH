// Browser-facing cron job contract, aligned with backend-v1/internal/webapi.
// nextRunAt is RFC3339 or null when the job has no next execution.

export type CronTaskType = "once" | "recurring";

export type CronJob = {
  id: string;
  name: string;
  taskType: CronTaskType;
  schedule: string;
  prompt: string;
  enabled: boolean;
  nextRunAt: string | null;
  running: boolean;
  createdAt: string;
};

export type CronJobListResponse = {
  jobs: CronJob[];
};

export type CreateCronJobRequest = {
  name: string;
  taskType: CronTaskType;
  // schedule: recurring 为标准 5 段 cron 表达式；once 为 RFC3339 时间，
  // 由前端从 datetime-local 的本地时间按浏览器时区自动转换后提交。
  schedule: string;
  prompt: string;
  timezone: string;
};

export type UpdateCronJobRequest = Omit<CreateCronJobRequest, "timezone">;