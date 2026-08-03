"use client";

import { useEffect, useRef, useState } from "react";

import type { SessionUsage } from "@/lib/chat/type";

type SessionUsageProps = {
  usage: SessionUsage;
  menuPosition?: "up" | "down";
};

export function SessionUsageView({ usage, menuPosition = "up" }: SessionUsageProps) {
  const [open, setOpen] = useState(false);
  const view = useRef<HTMLDivElement>(null);

  useEffect(() => {
    function closeOnOutsideClick(event: MouseEvent) {
      if (!view.current?.contains(event.target as Node)) {
        setOpen(false);
      }
    }

    document.addEventListener("mousedown", closeOnOutsideClick);
    return () => document.removeEventListener("mousedown", closeOnOutsideClick);
  }, []);

  return (
    <div className="relative" ref={view}>
      <button
        aria-expanded={open}
        className="flex h-7 items-center gap-1 rounded-md px-1.5 text-sm text-muted transition-colors hover:bg-surface-subtle hover:text-ink"
        onClick={() => setOpen((current) => !current)}
        type="button"
      >
        <span>{formatTokens(usage.totalTokens)} tokens</span>
        <Chevron open={open} />
      </button>

      {open && <div className={`absolute left-0 z-20 w-64 rounded-xl border border-border bg-surface p-3 shadow-xl shadow-black/10 ${menuPosition === "up" ? "bottom-7" : "top-7"}`}>
        <p className="mb-2 text-sm font-medium text-ink">本会话用量</p>
        <div className="space-y-2 text-sm">
          <UsageRow label="命中缓存" value={optionalTokens(usage.cachedPromptTokens)} />
          <UsageRow label="未命中缓存" value={optionalTokens(usage.uncachedPromptTokens)} />
          <UsageRow label="输出 Token" value={formatTokens(usage.completionTokens)} />
          <UsageRow label="缓存命中率" value={usage.cacheHitRate === null ? "—" : `${Math.round(usage.cacheHitRate * 100)}%`} />
        </div>
        {usage.cacheHitRate === null && <p className="mt-3 border-t border-border pt-2 text-xs leading-5 text-muted">部分模型未报告缓存数据</p>}
      </div>}
    </div>
  );
}

function Chevron({ open }: { open: boolean }) {
  return <span
    aria-hidden="true"
    className={`mb-0.5 inline-block h-1.5 w-1.5 rotate-45 border-b border-r border-current transition-transform ${open ? "rotate-[225deg]" : ""}`}
  />;
}

function UsageRow({ label, value }: { label: string; value: string }) {
  return <div className="flex items-center justify-between gap-4">
    <span className="text-muted">{label}</span>
    <span className="font-medium text-ink">{value}</span>
  </div>;
}

function optionalTokens(value: number | null) {
  return value === null ? "—" : formatTokens(value);
}

function formatTokens(value: number) {
  if (value < 1000) {
    return String(value);
  }
  return `${(value / 1000).toFixed(1).replace(/\.0$/, "")}k`;
}
