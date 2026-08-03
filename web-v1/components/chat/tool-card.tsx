import { Check, ChevronDown, CircleAlert, LoaderCircle, Wrench } from "lucide-react";
import { memo } from "react";

import type { ToolBlock } from "@/lib/chat/type";

export const ToolCard = memo(function ToolCard({ block }: { block: ToolBlock }) {
  const state = block.status === "running" ? "运行中" : block.status === "failed" ? "执行失败" : "已完成";
  const stateClass = block.status === "running"
    ? "bg-accent-soft text-accent"
    : block.status === "failed"
      ? "bg-danger-soft text-danger"
      : "bg-surface-hover text-muted";

  const StateIcon = block.status === "running" ? LoaderCircle : block.status === "failed" ? CircleAlert : Check;

  return (
    <details className="group overflow-hidden rounded-xl border border-border bg-surface text-xs leading-5 shadow-sm shadow-black/[0.02]">
      <summary className="flex cursor-pointer list-none items-center gap-2.5 px-3 py-2.5 hover:bg-surface-subtle [&::-webkit-details-marker]:hidden">
        <span className="inline-flex size-7 shrink-0 items-center justify-center rounded-lg bg-surface-subtle text-muted"><Wrench className="size-3.5" /></span>
        <span className="min-w-0 flex-1">
          <span className="block truncate font-mono text-[11px] font-medium text-ink">{block.toolName}</span>
          <span className="block text-[10px] text-faint">工具调用</span>
        </span>
        <span className={`inline-flex shrink-0 items-center gap-1 rounded-full px-2 py-1 text-[10px] font-medium ${stateClass}`}><StateIcon className={`size-3 ${block.status === "running" ? "animate-spin" : ""}`} />{state}</span>
        <ChevronDown className="size-3.5 shrink-0 text-faint transition-transform group-open:rotate-180" />
      </summary>
      <div className="space-y-3 border-t border-border bg-surface-subtle p-3">
        <div>
          <p className="mb-1 text-[11px] font-medium text-muted">调用参数</p>
          <pre className="max-h-48 overflow-auto rounded-lg bg-surface p-2.5 font-mono whitespace-pre-wrap text-muted">{block.arguments}</pre>
        </div>
        {block.result !== undefined && (
          <div>
            <p className="mb-1 text-[11px] font-medium text-muted">工具结果</p>
            <pre className="max-h-80 overflow-auto rounded-lg bg-surface p-2.5 font-mono whitespace-pre-wrap text-ink">{block.result}</pre>
          </div>
        )}
      </div>
    </details>
  );
});
