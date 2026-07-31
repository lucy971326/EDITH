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
  schedule: string;
  prompt: string;
  timezone: string;
};

export type UpdateCronJobRequest = Omit<CreateCronJobRequest, "timezone">;