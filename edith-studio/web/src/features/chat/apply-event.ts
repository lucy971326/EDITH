import type { AssistantBlock, StreamEvent } from "../../lib/stream";
import type { AssistantMessage } from "./types";

// applyEvent 把一条 StreamEvent 合并进 Assistant 消息，是 chat 领域唯一的事件归并逻辑。
// delta 事件按 blockId 追加到已有块；工具事件按 toolCallId 更新或新建工具卡片。
export function applyEvent(message: AssistantMessage, streamEvent: StreamEvent): AssistantMessage {
  if (streamEvent.type === "message.delta" || streamEvent.type === "reasoning.delta") {
    if (!streamEvent.blockId || !streamEvent.blockType || !streamEvent.delta) {
      return message;
    }
    const current = message.blocks.find((block) => block.id === streamEvent.blockId);
    if (current?.type === "text" || current?.type === "reasoning") {
      return {
        ...message,
        blocks: message.blocks.map((block) =>
          block.id === streamEvent.blockId && (block.type === "text" || block.type === "reasoning")
            ? { ...block, content: block.content + streamEvent.delta }
            : block,
        ),
      };
    }
    return {
      ...message,
      blocks: [...message.blocks, { id: streamEvent.blockId, type: streamEvent.blockType, content: streamEvent.delta }],
    };
  }
  if (!streamEvent.type.startsWith("tool.") || !streamEvent.toolCallId) {
    return message;
  }
  const current = message.blocks.find(
    (block): block is Extract<AssistantBlock, { type: "tool" }> => block.id === streamEvent.toolCallId && block.type === "tool",
  );
  const next: Extract<AssistantBlock, { type: "tool" }> = {
    id: streamEvent.toolCallId,
    type: "tool",
    name: streamEvent.toolName ?? current?.name ?? "工具",
    arguments: streamEvent.arguments ?? current?.arguments ?? "",
    result: streamEvent.toolResult ?? current?.result ?? "",
    status: streamEvent.toolStatus ?? current?.status ?? "requested",
  };
  return {
    ...message,
    blocks: current ? message.blocks.map((block) => (block.id === next.id ? next : block)) : [...message.blocks, next],
  };
}
