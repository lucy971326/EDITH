import { memo, useEffect, useRef } from "react";

import type { Timeline } from "@/lib/chat/type";

import { AssistantBlockView } from "./assistant-block";
import { UserBlockView } from "./user-block";

export type TimelineRunNotice = "stream_disconnected" | "session_busy";

type TimelineViewProps = {
  timeline: Timeline;
  runNotice?: TimelineRunNotice;
};

export const TimelineView = memo(function TimelineView({ timeline, runNotice }: TimelineViewProps) {
  const scrollRef = useRef<HTMLDivElement>(null);
  // 用户停留在底部时自动跟随新消息；向上翻阅历史时暂停跟随。
  const stickToBottom = useRef(true);

  useEffect(() => {
    const container = scrollRef.current;
    if (!container || !stickToBottom.current) return;
    container.scrollTop = container.scrollHeight;
  }, [timeline]);

  function handleScroll() {
    const container = scrollRef.current;
    if (!container) return;
    const distanceFromBottom = container.scrollHeight - container.scrollTop - container.clientHeight;
    stickToBottom.current = distanceFromBottom < 80;
  }
  return (
    <div className="min-h-0 flex-1 overflow-y-auto" onScroll={handleScroll} ref={scrollRef}>
      <div className="mx-auto max-w-4xl space-y-7 px-6 py-8 md:px-8">
        {timeline.blocks.map((block) => {
          if (block.type === "user") {
            return <UserBlockView block={block} key={block.id} />;
          }
          if (block.type === "assistant") {
            return <AssistantBlockView block={block} key={block.id} />;
          }
          return (
            <p className="text-sm text-danger" key={block.id}>
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
    <div className="flex items-center gap-3 rounded-xl border border-border bg-surface-subtle px-4 py-3 text-sm text-muted" role="status">
      <span aria-hidden className="h-2 w-2 shrink-0 animate-pulse rounded-full bg-accent" />
      <div>
        <p className="font-medium text-ink">{copy.title}</p>
        <p className="mt-0.5 text-xs text-muted">{copy.description}</p>
      </div>
    </div>
  );
}
