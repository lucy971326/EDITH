"use client";

import { useEffect, useState } from "react";

import { PageSidebar } from "@/components/page-sidebar";
import type { CronJob, CronJobListResponse, CronTaskType } from "@/lib/cron/type";

type FormState = {
  name: string;
  taskType: CronTaskType;
  schedule: string;
  prompt: string;
};

const emptyForm: FormState = { name: "", taskType: "recurring", schedule: "", prompt: "" };

// browserTimezone 是创建任务时写入用户设置的时区来源。
function browserTimezone(): string {
  try {
    return Intl.DateTimeFormat().resolvedOptions().timeZone || "Asia/Shanghai";
  } catch {
    return "Asia/Shanghai";
  }
}

// localDateTimeFromNow 生成 N 分钟后的本地时间，格式适配 datetime-local 输入。
function localDateTimeFromNow(minutes: number): string {
  const date = new Date(Date.now() + minutes * 60_000);
  const pad = (value: number) => String(value).padStart(2, "0");
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(date.getHours())}:${pad(date.getMinutes())}`;
}

// localToRFC3339 把 datetime-local 的本地时间转换为带时区偏移的 RFC3339。
// 后端契约要求一次性任务的 schedule 是 RFC3339；浏览器时区就是用户看到的时间语义。
function localToRFC3339(value: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  const offsetMinutes = -date.getTimezoneOffset();
  const sign = offsetMinutes >= 0 ? "+" : "-";
  const absolute = Math.abs(offsetMinutes);
  const pad = (value: number) => String(value).padStart(2, "0");
  return `${value}:00${sign}${pad(Math.floor(absolute / 60))}:${pad(absolute % 60)}`;
}

function formatNextRun(value: string | null): string {
  if (!value) return "—";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleString("zh-CN", { hour12: false });
}

export function CronJobsPage() {
  const [jobs, setJobs] = useState<CronJob[]>([]);
  const [form, setForm] = useState<FormState>(emptyForm);
  const [error, setError] = useState("");
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    let stopped = false;
    async function load() {
      try {
        const response = await fetch("/api/cron-jobs", { cache: "no-store" });
        if (!response.ok) return;
        const data = (await response.json()) as CronJobListResponse;
        if (!stopped) setJobs(data.jobs);
      } catch {
        // 加载失败保持空列表，不阻塞页面。
      }
    }
    void load();
    return () => {
      stopped = true;
    };
  }, []);

  async function createJob() {
    if (!form.name.trim() || !form.schedule.trim() || !form.prompt.trim()) {
      setError("请填写任务名、执行时间和任务指令。");
      return;
    }
    setSaving(true);
    setError("");
    try {
      const schedule = form.taskType === "once" ? localToRFC3339(form.schedule) : form.schedule.trim();
      const response = await fetch("/api/cron-jobs", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ ...form, schedule, timezone: browserTimezone() }),
      });
      if (!response.ok) {
        setError((await response.text()) || "创建失败。");
        return;
      }
      setForm(emptyForm);
      const created = (await response.json()) as CronJob;
      setJobs((current) => [...current, created]);
    } catch {
      setError("创建失败，请稍后重试。");
    } finally {
      setSaving(false);
    }
  }

  async function toggleJob(job: CronJob) {
    try {
      const response = await fetch(`/api/cron-jobs/${encodeURIComponent(job.id)}`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ enabled: !job.enabled }),
      });
      if (!response.ok) return;
      const updated = (await response.json()) as CronJob;
      setJobs((current) => current.map((item) => (item.id === updated.id ? updated : item)));
    } catch {
      // 静默失败，保持当前状态。
    }
  }

  async function deleteJob(job: CronJob) {
    if (!window.confirm(`删除定时任务「${job.name}」？`)) return;
    try {
      const response = await fetch(`/api/cron-jobs/${encodeURIComponent(job.id)}`, { method: "DELETE" });
      if (!response.ok) return;
      setJobs((current) => current.filter((item) => item.id !== job.id));
    } catch {
      // 静默失败，保持当前列表。
    }
  }

  const scheduleHint =
    form.taskType === "once"
      ? "一次性任务：选择触发时间，或点“5 分钟后 / 1 小时后”自动填充"
      : "周期性任务：填标准 cron 表达式，如 0 9 * * *（每天 9 点）";

  return (
    <main className="flex h-screen overflow-hidden bg-zinc-50">
      <PageSidebar activePage="tasks" />
      <section className="min-w-0 flex-1 overflow-y-auto p-6">
      <header className="mb-6 flex items-center justify-between">
        <div>
          <h1 className="text-lg font-semibold text-zinc-900">定时任务</h1>
          <p className="mt-0.5 text-sm text-zinc-500">到点后由 EDITH 自动执行，结果保存在对应会话中</p>
        </div>
        <a className="rounded-lg border border-zinc-300 bg-white px-3 py-1.5 text-sm text-zinc-700 hover:bg-zinc-100" href="/chat">
          返回对话
        </a>
      </header>

      <section className="mb-8 rounded-xl border border-zinc-200 bg-white p-5">
        <h2 className="mb-4 text-sm font-medium text-zinc-900">新建任务</h2>
        <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
          <label className="block">
            <span className="mb-1 block text-xs text-zinc-500">任务名</span>
            <input
              className="w-full rounded-lg border border-zinc-300 px-3 py-2 text-sm outline-none focus:border-zinc-500"
              onChange={(event) => setForm((current) => ({ ...current, name: event.target.value }))}
              placeholder="例如：每日晨报"
              value={form.name}
            />
          </label>
          <label className="block">
            <span className="mb-1 block text-xs text-zinc-500">任务类型</span>
            <select
              className="w-full rounded-lg border border-zinc-300 px-3 py-2 text-sm outline-none focus:border-zinc-500"
              onChange={(event) => setForm((current) => ({ ...current, taskType: event.target.value as CronTaskType, schedule: "" }))}
              value={form.taskType}
            >
              <option value="recurring">周期性任务</option>
              <option value="once">一次性任务</option>
            </select>
          </label>
          <label className="block">
            <span className="mb-1 block text-xs text-zinc-500">执行时间</span>
            {form.taskType === "once" ? (
              <input
                className="w-full rounded-lg border border-zinc-300 px-3 py-2 text-sm outline-none focus:border-zinc-500"
                min={localDateTimeFromNow(0)}
                onChange={(event) => setForm((current) => ({ ...current, schedule: event.target.value }))}
                type="datetime-local"
                value={form.schedule}
              />
            ) : (
              <input
                className="w-full rounded-lg border border-zinc-300 px-3 py-2 text-sm outline-none focus:border-zinc-500"
                onChange={(event) => setForm((current) => ({ ...current, schedule: event.target.value }))}
                placeholder="0 9 * * *"
                value={form.schedule}
              />
            )}
            {form.taskType === "once" && (
              <span className="mt-2 flex gap-2">
                <button
                  className="rounded-lg border border-zinc-300 px-2.5 py-1 text-xs text-zinc-600 transition-colors hover:bg-zinc-100"
                  onClick={() => setForm((current) => ({ ...current, schedule: localDateTimeFromNow(5) }))}
                  type="button"
                >
                  5 分钟后
                </button>
                <button
                  className="rounded-lg border border-zinc-300 px-2.5 py-1 text-xs text-zinc-600 transition-colors hover:bg-zinc-100"
                  onClick={() => setForm((current) => ({ ...current, schedule: localDateTimeFromNow(60) }))}
                  type="button"
                >
                  1 小时后
                </button>
              </span>
            )}
          </label>
          <label className="block">
            <span className="mb-1 block text-xs text-zinc-500">任务指令</span>
            <input
              className="w-full rounded-lg border border-zinc-300 px-3 py-2 text-sm outline-none focus:border-zinc-500"
              onChange={(event) => setForm((current) => ({ ...current, prompt: event.target.value }))}
              placeholder="例如：总结昨天的数据并生成日报"
              value={form.prompt}
            />
          </label>
        </div>
        <p className="mt-2 text-xs text-zinc-400">{scheduleHint}。时区使用当前浏览器时区（{browserTimezone()}）。</p>
        {error ? <p className="mt-3 text-sm text-red-600">{error}</p> : null}
        <button
          className="mt-4 rounded-lg bg-zinc-900 px-4 py-2 text-sm text-white hover:bg-zinc-700 disabled:cursor-not-allowed disabled:opacity-50"
          disabled={saving}
          onClick={() => void createJob()}
          type="button"
        >
          {saving ? "创建中…" : "创建任务"}
        </button>
      </section>

      <section className="rounded-xl border border-zinc-200 bg-white">
        <div className="flex items-center justify-between border-b border-zinc-100 px-5 py-3">
          <h2 className="text-sm font-medium text-zinc-900">任务列表</h2>
          <span className="text-xs text-zinc-400">{jobs.length} 个任务</span>
        </div>
        {jobs.length === 0 ? (
          <p className="px-5 py-10 text-center text-sm text-zinc-400">还没有定时任务</p>
        ) : (
          <ul className="divide-y divide-zinc-100">
            {jobs.map((job) => (
              <li className="flex items-center gap-4 px-5 py-3" key={job.id}>
                <div className="min-w-0 flex-1">
                  <div className="flex items-center gap-2">
                    <span className="text-sm font-medium text-zinc-900">{job.name}</span>
                    <span className={`rounded px-1.5 py-0.5 text-xs ${job.enabled ? "bg-emerald-50 text-emerald-600" : "bg-zinc-100 text-zinc-500"}`}>
                      {job.enabled ? "启用" : "停用"}
                    </span>
                    {job.running ? <span className="rounded bg-amber-50 px-1.5 py-0.5 text-xs text-amber-600">运行中</span> : null}
                  </div>
                  <p className="mt-0.5 truncate text-xs text-zinc-500">
                    {job.taskType === "once" ? "一次性" : "周期"} · {job.schedule} · 下次：{formatNextRun(job.nextRunAt)}
                  </p>
                  <p className="mt-0.5 truncate text-xs text-zinc-400">{job.prompt}</p>
                </div>
                <button
                  className="rounded-lg border border-zinc-300 px-3 py-1.5 text-xs text-zinc-700 hover:bg-zinc-100"
                  onClick={() => void toggleJob(job)}
                  type="button"
                >
                  {job.enabled ? "停用" : "启用"}
                </button>
                <button
                  className="rounded-lg border border-red-200 px-3 py-1.5 text-xs text-red-600 hover:bg-red-50"
                  onClick={() => void deleteJob(job)}
                  type="button"
                >
                  删除
                </button>
              </li>
            ))}
          </ul>
        )}
      </section>
      </section>
    </main>
  );
}
