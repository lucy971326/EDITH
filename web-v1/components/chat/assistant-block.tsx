import { BrainCircuit, ChevronDown } from "lucide-react";
import { memo } from "react";

import type { AssistantBlock } from "@/lib/chat/type";

import { MessageMarkdown } from "./message-markdown";
import { ToolCard } from "./tool-card";

export const AssistantBlockView = memo(function AssistantBlockView({ block }: { block: AssistantBlock }) {
  return (
    <article className="space-y-4 text-sm leading-7 text-ink">
      {block.blocks.map((content) => {
        if (content.type === "reasoning") {
          return (
            <details className="group rounded-xl border border-border bg-surface text-muted shadow-sm shadow-black/[0.02]" key={content.id}>
              <summary className="flex cursor-pointer list-none items-center gap-2 px-3 py-2.5 text-xs font-medium select-none [&::-webkit-details-marker]:hidden">
                <span className="inline-flex size-6 items-center justify-center rounded-lg bg-accent-soft text-accent"><BrainCircuit className="size-3.5" /></span>
                <span className="text-ink">思考过程</span>
                <span className="truncate text-faint">模型推理内容</span>
                <ChevronDown className="ml-auto size-3.5 shrink-0 text-faint transition-transform group-open:rotate-180" />
              </summary>
              <p className="border-t border-border px-3 py-3 whitespace-pre-wrap text-xs leading-6 text-muted">{content.content}</p>
            </details>
          );
        }
        if (content.type === "tool") {
          return <ToolCard block={content} key={content.id} />;
        }
        return <MessageMarkdown content={content.content} key={content.id} />;
      })}
    </article>
  );
});
