import type {
  AssistantBlock,
  ChatStreamEvent,
  ErrorBlock,
  Timeline,
} from "./type";

// Read EDITH's SSE frames without exposing transport details to ChatPage.
export async function readChatStream(
  response: Response,
  onEvent: (event: ChatStreamEvent) => void,
) {
  if (!response.body) {
    throw new Error("EDITH did not return a response stream");
  }

  const reader = response.body.getReader();
  const decoder = new TextDecoder();
  let buffer = "";

  while (true) {
    const { done, value } = await reader.read();
    buffer += decoder.decode(value, { stream: !done });

    let boundary = buffer.indexOf("\n\n");
    while (boundary >= 0) {
      const frame = buffer.slice(0, boundary);
      buffer = buffer.slice(boundary + 2);
      readFrame(frame, onEvent);
      boundary = buffer.indexOf("\n\n");
    }

    if (done) {
      return;
    }
  }
}

// applyStreamEvent is the single bridge from an SSE event to visible state.
export function applyStreamEvent(timeline: Timeline, event: ChatStreamEvent): Timeline {
  if (event.type === "run.started") {
    return {
      blocks: [...timeline.blocks, {
        type: "assistant",
        id: event.assistantId,
        createdAt: new Date().toISOString(),
        blocks: [],
      }],
    };
  }

  if (event.type === "run.error") {
    return { blocks: [...timeline.blocks, errorTimelineEvent(event.error.message)] };
  }

  if (event.type === "reasoning.delta" || event.type === "message.delta") {
    return updateAssistant(timeline, event.assistantId, (assistant) => {
      const index = assistant.blocks.findIndex((block) => block.id === event.blockId);
      if (index < 0) {
        return {
          ...assistant,
          blocks: [
            ...assistant.blocks,
            { type: event.blockType, id: event.blockId, content: event.delta },
          ],
        };
      }

      const blocks = [...assistant.blocks];
      const block = blocks[index];
      if (block.type === "tool") {
        return assistant;
      }
      blocks[index] = { ...block, content: block.content + event.delta };
      return { ...assistant, blocks };
    });
  }

  if (event.type === "tool.started") {
    return updateAssistant(timeline, event.assistantId, (assistant) => ({
      ...assistant,
      blocks: [...assistant.blocks, {
        type: "tool",
        id: event.toolCallId,
        toolName: event.toolName,
        arguments: event.arguments ?? "",
        status: event.toolStatus,
      }],
    }));
  }

  if (event.type === "tool.finished") {
    return updateAssistant(timeline, event.assistantId, (assistant) => ({
      ...assistant,
      blocks: assistant.blocks.map((block) =>
        block.id === event.toolCallId
          ? { ...block, status: event.toolStatus, result: event.toolResult }
          : block,
      ),
    }));
  }

  return timeline;
}

export function errorTimelineEvent(message: string): ErrorBlock {
  return {
    type: "error",
    id: crypto.randomUUID(),
    message,
    createdAt: new Date().toISOString(),
  };
}

function readFrame(frame: string, onEvent: (event: ChatStreamEvent) => void) {
  const data = frame
    .split("\n")
    .filter((line) => line.startsWith("data: "))
    .map((line) => line.slice("data: ".length))
    .join("\n");
  if (!data) {
    return;
  }

  onEvent(JSON.parse(data) as ChatStreamEvent);
}

function updateAssistant(
  timeline: Timeline,
  assistantID: string,
  update: (assistant: AssistantBlock) => AssistantBlock,
): Timeline {
  return {
    blocks: timeline.blocks.map((block) =>
      block.type === "assistant" && block.id === assistantID ? update(block) : block,
    ),
  };
}
