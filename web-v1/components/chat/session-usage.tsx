"use client";

import { useEffect, useRef, useState } from "react";

import type { SessionUsage } from "@/lib/chat/type";

type SessionUsageProps = {
  usage: SessionUsage;
};

export function SessionUsageView({ usage }: SessionUsageProps) {
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
        className="flex items-center gap-1.5 rounded-md px-1 text-xs text-zinc-500 transition-colors hover:text-zinc-800"
        onClick={() => setOpen((current) => !current)}
        type="button"
      >
        <span>本会话 · {formatTokens(usage.totalTokens)} tokens</span>
        <Chevron open={open} />
      </button>

      {open && <div className="absolute bottom-7 left-0 z-20 w-64 rounded-xl border border-zinc-200 bg-white p-3 shadow-xl shadow-zinc-900/10">
        <p className="mb-2 text-sm font-medium text-zinc-900">本会话用量</p>
        <div className="space-y-2 text-sm">
          <UsageRow label="命中缓存" value={optionalTokens(usage.cachedPromptTokens)} />
          <UsageRow label="未命中缓存" value={optionalTokens(usage.uncachedPromptTokens)} />
          <UsageRow label="输出 Token" value={formatTokens(usage.completionTokens)} />
          <UsageRow label="缓存命中率" value={usage.cacheHitRate === null ? "—" : `${Math.round(usage.cacheHitRate * 100)}%`} />
        </div>
        {usage.cacheHitRate === null && <p className="mt-3 border-t border-zinc-100 pt-2 text-xs leading-5 text-zinc-500">部分模型未报告缓存数据</p>}
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
    <span className="text-zinc-500">{label}</span>
    <span className="font-medium text-zinc-800">{value}</span>
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
