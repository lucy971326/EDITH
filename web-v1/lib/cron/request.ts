import type { CreateCronJobRequest, CronTaskType } from "./type";

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

// parseCronJobRequest is the browser-facing cron job boundary before
// Clerk's userId is added by the BFF route.
export function parseCronJobRequest(value: unknown): CreateCronJobRequest | null {
  if (
    !isRecord(value) ||
    typeof value.name !== "string" || !value.name.trim() ||
    (value.taskType !== "once" && value.taskType !== "recurring") ||
    typeof value.schedule !== "string" || !value.schedule.trim() ||
    typeof value.prompt !== "string" || !value.prompt.trim() ||
    typeof value.timezone !== "string" || !value.timezone.trim()
  ) return null;

  return {
    name: value.name.trim(),
    taskType: value.taskType as CronTaskType,
    schedule: value.schedule.trim(),
    prompt: value.prompt.trim(),
    timezone: value.timezone.trim(),
  };
}