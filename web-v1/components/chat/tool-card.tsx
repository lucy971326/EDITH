import type { ToolBlock } from "@/lib/chat/type";

export const ToolCard = memo(function ToolCard({ block }: { block: ToolBlock }) {
  const state = block.status === "running" ? "调用中" : block.status === "failed" ? "失败" : "已完成";
  const stateClass = block.status === "running"
    ? "bg-zinc-200 text-zinc-600"
    : block.status === "failed"
      ? "bg-zinc-200 text-zinc-700"
      : "bg-zinc-200 text-zinc-500";

  return (
    <details className="group overflow-hidden rounded-lg bg-zinc-100 text-xs leading-5">
      <summary className="cursor-pointer px-3 py-2 hover:bg-zinc-200/60">
        <span className={`float-right rounded-full px-2 py-0.5 text-[11px] font-medium ${stateClass}`}>{state}</span>
        <span className="font-mono text-zinc-700">{block.toolName}</span>
      </summary>
      <div className="space-y-3 border-t border-zinc-200 bg-zinc-100 p-3">
        <div>
          <p className="mb-1 text-[11px] font-medium text-zinc-500">调用参数</p>
          <pre className="max-h-48 overflow-auto rounded-lg bg-zinc-50 p-2.5 font-mono whitespace-pre-wrap text-zinc-600">{block.arguments}</pre>
        </div>
        {block.result !== undefined && (
          <div>
            <p className="mb-1 text-[11px] font-medium text-zinc-500">工具结果</p>
            <pre className="max-h-80 overflow-auto rounded-lg bg-zinc-50 p-2.5 font-mono whitespace-pre-wrap text-zinc-800">{block.result}</pre>
          </div>
        )}
      </div>
    </details>
  );
});
import { memo } from "react";
