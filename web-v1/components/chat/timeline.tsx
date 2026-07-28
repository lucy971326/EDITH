import type { Timeline } from "@/lib/chat/type";

import { AssistantBlockView } from "./assistant-block";
import { UserBlockView } from "./user-block";

type TimelineViewProps = {
  timeline: Timeline;
};

export const TimelineView = memo(function TimelineView({ timeline }: TimelineViewProps) {
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
      </div>
    </div>
  );
});
import { memo } from "react";
