import type { ToolBlock } from "@/lib/chat/type";

export function ToolCard({ block }: { block: ToolBlock }) {
  const state = block.status === "running" ? "调用中" : block.status === "failed" ? "失败" : "已完成";

  return (
    <section className="overflow-hidden rounded-lg border border-zinc-200 bg-zinc-50 text-xs leading-5">
      <div className="flex items-center justify-between border-b border-zinc-200 px-3 py-2">
        <span className="font-mono text-zinc-700">{block.toolName}</span>
        <span className="text-zinc-500">{state}</span>
      </div>
      <div className="space-y-2 p-3 text-zinc-600">
        <pre className="overflow-x-auto whitespace-pre-wrap font-mono">{block.arguments}</pre>
        {block.result && (
          <pre className="overflow-x-auto whitespace-pre-wrap border-t border-zinc-200 pt-2 font-mono text-zinc-800">
            {block.result}
          </pre>
        )}
      </div>
    </section>
  );
}
