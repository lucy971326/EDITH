import { memo } from "react";

import type { Timeline } from "@/lib/chat/type";

import { AssistantBlockView } from "./assistant-block";
import { UserBlockView } from "./user-block";

export type TimelineRunNotice = "stream_disconnected" | "session_busy";

type TimelineViewProps = {
  timeline: Timeline;
  runNotice?: TimelineRunNotice;
};

export const TimelineView = memo(function TimelineView({ timeline, runNotice }: TimelineViewProps) {
  return (
    <div className="min-h-0 flex-1 overflow-y-auto">
      <div className="mx-auto max-w-3xl space-y-8 px-5 py-8">
        {timeline.blocks.map((block) => {
          if (block.type === "user") {
            return <UserBlockView block={block} key={block.id} />;
          }
          if (block.type === "assistant") {
            return <AssistantBlockView block={block} key={block.id} />;
          }
          return (
            <p className="text-sm text-red-600" key={block.id}>
              {block.message}
            </p>
          );
        })}
        {runNotice && <ActiveRunNotice reason={runNotice} />}
      </div>
    </div>
  );
});

function ActiveRunNotice({ reason }: { reason: TimelineRunNotice }) {
  const copy = reason === "stream_disconnected"
    ? {
        title: "任务仍在后台执行",
        description: "当前页面未连接实时输出；完成后会自动加载完整结果。",
      }
    : {
        title: "该会话正在生成回复",
        description: "已有一条任务正在执行；完成后会自动显示完整结果。",
      };

  return (
    <div className="flex items-center gap-3 rounded-xl border border-zinc-200 bg-zinc-50 px-4 py-3 text-sm text-zinc-600" role="status">
      <span aria-hidden className="h-2 w-2 shrink-0 animate-pulse rounded-full bg-zinc-400" />
      <div>
        <p className="font-medium text-zinc-700">{copy.title}</p>
        <p className="mt-0.5 text-xs text-zinc-500">{copy.description}</p>
      </div>
    </div>
  );
}
