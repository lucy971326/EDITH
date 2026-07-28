import type { AssistantBlock } from "@/lib/chat/type";

import { MessageMarkdown } from "./message-markdown";
import { ToolCard } from "./tool-card";

export function AssistantBlockView({ block }: { block: AssistantBlock }) {
  return (
    <article className="space-y-4 text-sm leading-7 text-zinc-800">
      {block.blocks.map((content) => {
        if (content.type === "reasoning") {
          return (
            <details className="rounded-lg bg-zinc-100 px-3 py-2 text-zinc-500" key={content.id}>
              <summary className="cursor-pointer text-xs font-medium select-none">思考过程</summary>
              <p className="mt-2 whitespace-pre-wrap text-xs leading-6">{content.content}</p>
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
}
