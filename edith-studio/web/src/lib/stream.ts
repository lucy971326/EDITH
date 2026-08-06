export type ToolStatus = "requested" | "running" | "completed" | "failed";

export type StreamEvent = {
  type: string;
  blockId?: string;
  blockType?: "reasoning" | "text";
  delta?: string;
  toolCallId?: string;
  toolName?: string;
  arguments?: string;
  toolStatus?: ToolStatus;
  toolResult?: string;
  error?: { message: string };
};

export type AssistantBlock =
  | { id: string; type: "reasoning"; content: string }
  | { id: string; type: "text"; content: string }
  | { id: string; type: "tool"; name: string; arguments: string; result: string; status: ToolStatus };

export function readSSEFrames(buffer: string): { events: StreamEvent[]; rest: string } {
  const frames = buffer.split(/\r?\n\r?\n/);
  const rest = frames.pop() ?? "";
  const events = frames.flatMap((frame) => {
    const data = frame
      .split(/\r?\n/)
      .find((line) => line.startsWith("data: "))
      ?.slice(6);
    if (!data) {
      return [];
    }
    return [JSON.parse(data) as StreamEvent];
  });
  return { events, rest };
}
