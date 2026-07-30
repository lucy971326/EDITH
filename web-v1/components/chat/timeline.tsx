import { memo } from "react";

import type { Timeline } from "@/lib/chat/type";

import { AssistantBlockView } from "./assistant-block";
import { UserBlockView } from "./user-block";

type TimelineViewProps = {
  timeline: Timeline;
  backgroundTaskRunning: boolean;
};

export const TimelineView = memo(function TimelineView({ timeline, backgroundTaskRunning }: TimelineViewProps) {
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
        {backgroundTaskRunning && <BackgroundTaskNotice />}
      </div>
    </div>
  );
});

function BackgroundTaskNotice() {
  return (
    <div className="flex items-center gap-3 rounded-xl border border-zinc-200 bg-zinc-50 px-4 py-3 text-sm text-zinc-600" role="status">
      <span aria-hidden className="h-2 w-2 shrink-0 animate-pulse rounded-full bg-zinc-400" />
      <div>
        <p className="font-medium text-zinc-700">任务仍在后台执行</p>
        <p className="mt-0.5 text-xs text-zinc-500">网络恢复后会自动显示完整结果</p>
      </div>
    </div>
  );
}
