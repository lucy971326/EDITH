import type { AssistantBlock, StreamEvent } from "../../lib/stream";
import type { AssistantMessage } from "./types";

// applyEvent 把一条 StreamEvent 合并进 Assistant 消息，是 chat 领域唯一的事件归并逻辑。
// 父事件（无 author）进顶层块；子 Agent 事件（有 author）归入对应的 Agent Card。
export function applyEvent(message: AssistantMessage, streamEvent: StreamEvent): AssistantMessage {
  if (streamEvent.author) {
    return applySubEvent(message, streamEvent);
  }
  return { ...message, blocks: applyBlocks(message.blocks, streamEvent) };
}

// applySubEvent 把子 Agent 事件归入对应 Agent Card；
// 若父 tool 卡还未升级为 Agent Card（第一条子事件），就地升级并归入。
function applySubEvent(message: AssistantMessage, streamEvent: StreamEvent): AssistantMessage {
  const index = findAgentIndex(message.blocks, streamEvent.parentToolCallId);
  if (index < 0) {
    return message;
  }
  const block = message.blocks[index];
  const next: AssistantBlock =
    block.type === "agent"
      ? { ...block, blocks: applyBlocks(block.blocks, streamEvent) }
      : // findAgentIndex 已保证该块是 tool 或 agent。
        upgradeToAgent(block as Extract<AssistantBlock, { type: "tool" }>, streamEvent);
  return { ...message, blocks: message.blocks.map((b, i) => (i === index ? next : b)) };
}

// findAgentIndex 定位子事件对应的卡片：优先按父 toolCallId 精确匹配；
// 兜底归入最后一个运行中的 Agent Card（单层单子场景）。
function findAgentIndex(blocks: AssistantBlock[], parentToolCallId?: string): number {
  if (parentToolCallId) {
    const exact = blocks.findIndex(
      (block) => block.id === parentToolCallId && (block.type === "agent" || block.type === "tool"),
    );
    if (exact >= 0) return exact;
  }
  for (let index = blocks.length - 1; index >= 0; index--) {
    const block = blocks[index];
    if (block.type === "agent" && block.status === "running") return index;
  }
  return -1;
}

// upgradeToAgent 把父的 agent 工具卡升级为 Agent Card（同一 id，保持状态连续）。
function upgradeToAgent(
  tool: Extract<AssistantBlock, { type: "tool" }>,
  streamEvent: StreamEvent,
): Extract<AssistantBlock, { type: "agent" }> {
  return {
    id: tool.id,
    type: "agent",
    name: tool.name,
    arguments: tool.arguments,
    status: "running",
    blocks: applyBlocks([], streamEvent),
  };
}

// applyBlocks 把一条事件合并进任意 blocks 数组（顶层或 Agent Card 内部）。
function applyBlocks(blocks: AssistantBlock[], streamEvent: StreamEvent): AssistantBlock[] {
  if (streamEvent.type === "message.delta" || streamEvent.type === "reasoning.delta") {
    if (!streamEvent.blockId || !streamEvent.blockType || !streamEvent.delta) {
      return blocks;
    }
    const current = blocks.find((block) => block.id === streamEvent.blockId && (block.type === "text" || block.type === "reasoning"));
    if (current) {
      return blocks.map((block) =>
        block.id === streamEvent.blockId && (block.type === "text" || block.type === "reasoning")
          ? { ...block, content: block.content + streamEvent.delta }
          : block,
      );
    }
    return [...blocks, { id: streamEvent.blockId, type: streamEvent.blockType, content: streamEvent.delta }];
  }
  if (!streamEvent.type.startsWith("tool.") || !streamEvent.toolCallId) {
    return blocks;
  }
  // 对应块已是 Agent Card（子事件已升级）：tool.finished 填 result 与终态。
  const agent = blocks.find(
    (block): block is Extract<AssistantBlock, { type: "agent" }> => block.type === "agent" && block.id === streamEvent.toolCallId,
  );
  if (agent) {
    if (streamEvent.type !== "tool.finished") {
      return blocks;
    }
    return blocks.map((block) =>
      block.id === streamEvent.toolCallId && block.type === "agent"
        ? { ...block, result: streamEvent.toolResult ?? block.result, status: streamEvent.toolStatus === "failed" ? "failed" : "completed" }
        : block,
    );
  }
  // 普通工具卡片逻辑（父的直接工具调用）。
  const current = blocks.find(
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
  return current ? blocks.map((block) => (block.id === next.id ? next : block)) : [...blocks, next];
}
