import type { AgentEvent, ChatMessage } from "@/types/api";

// 这个文件只负责一件事：把 SSE 事件变成按发生顺序展示的聊天消息。
export function applyAgentEvent(
  messages: ChatMessage[],
  event: AgentEvent,
): ChatMessage[] {
  const next = [...messages];

  switch (event.type) {
    case "text":
      return appendAssistantText(next, event.response_id, event.text);
    case "reasoning":
      return appendReasoning(next, event.response_id, event.thinking);
    case "tool_call":
      return addToolCall(next, event);
    case "tool_result":
      return addToolResult(next, event.id, event.name, event.result);
    case "error":
      return [
        ...next,
        { id: crypto.randomUUID(), kind: "error", text: event.message },
      ];
    case "done":
      return finishStreamingMessages(next);
  }
}

export function finishStreamingMessages(messages: ChatMessage[]): ChatMessage[] {
  return messages.map((message) =>
    message.kind === "assistant" && !message.done
      ? { ...message, done: true }
      : message,
  );
}

function appendAssistantText(
  messages: ChatMessage[],
  responseID: string,
  text: string,
): ChatMessage[] {
  const index = findLast(messages, (message) =>
    message.kind === "assistant" && message.response_id === responseID,
  );
  if (index >= 0) {
    const message = messages[index];
    if (message.kind === "assistant") {
      messages[index] = { ...message, text: message.text + text };
    }
    return messages;
  }

  return [
    ...messages,
    {
      id: crypto.randomUUID(),
      response_id: responseID,
      kind: "assistant",
      text,
      done: false,
    },
  ];
}

function appendReasoning(
  messages: ChatMessage[],
  responseID: string,
  text: string,
): ChatMessage[] {
  const index = findLast(messages, (message) =>
    message.kind === "reasoning" && message.response_id === responseID,
  );
  if (index >= 0) {
    const message = messages[index];
    if (message.kind === "reasoning") {
      messages[index] = { ...message, text: message.text + text };
    }
    return messages;
  }

  return [
    ...messages,
    {
      id: crypto.randomUUID(),
      response_id: responseID,
      kind: "reasoning",
      text,
    },
  ];
}

function addToolCall(
  messages: ChatMessage[],
  event: Extract<AgentEvent, { type: "tool_call" }>,
): ChatMessage[] {
  // 一个 Response 要开始执行工具时，它的文字阶段已经结束。
  const next = messages.map((message) =>
    message.kind === "assistant" && message.response_id === event.response_id
      ? { ...message, done: true }
      : message,
  );
  return [
    ...next,
    {
      id: crypto.randomUUID(),
      response_id: event.response_id,
      kind: "tool",
      tool_id: event.id,
      name: event.name,
      arguments: event.arguments,
    },
  ];
}

function addToolResult(
  messages: ChatMessage[],
  toolID: string,
  name: string,
  result: unknown,
): ChatMessage[] {
  const exactIndex = findLast(
    messages,
    (message) => message.kind === "tool" && message.tool_id === toolID,
  );
  const fallbackIndex = findLast(
    messages,
    (message) =>
      message.kind === "tool" && message.name === name && message.result === undefined,
  );
  const index = exactIndex >= 0 ? exactIndex : fallbackIndex;
  if (index < 0) return messages;

  const tool = messages[index];
  if (tool.kind === "tool") messages[index] = { ...tool, result };
  return messages;
}

function findLast<T>(items: T[], predicate: (item: T) => boolean): number {
  for (let index = items.length - 1; index >= 0; index--) {
    if (predicate(items[index])) return index;
  }
  return -1;
}
