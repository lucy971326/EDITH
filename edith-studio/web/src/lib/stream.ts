export type ToolStatus = "requested" | "running" | "completed" | "failed";

export type AgentStatus = "running" | "completed" | "failed";

export type StreamEvent = {
  type: string;
  author?: string; // 空 = 父 Agent；非空 = 子 Agent 名（如 explorer）
  parentToolCallId?: string; // 子事件触发的父 AgentTool 调用 toolCallId
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
  | { id: string; type: "error"; content: string }
  | { id: string; type: "tool"; name: string; arguments: string; result: string; status: ToolStatus }
  | {
      id: string;
      type: "agent";
      name: string;
      arguments: string; // 父调用参数 JSON（含 request），渲染时解析为任务
      status: AgentStatus;
      result?: string;
      blocks: AssistantBlock[]; // 子 Agent 的执行过程（思考/子工具/文本）
    };

// readSSEFrames 从缓冲字符串中切出完整 SSE 帧并解析为 StreamEvent；未结束的尾部留在 rest。
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
    try {
      return [JSON.parse(data) as StreamEvent];
    } catch {
      // 单帧解析失败不中断整个事件流，跳过这一帧继续读后续事件。
      return [];
    }
  });
  return { events, rest };
}

// consumeSSE 读取一个流式响应的完整 body，逐帧解析后交给 onEvent。
export async function consumeSSE(response: Response, onEvent: (event: StreamEvent) => void): Promise<void> {
  if (!response.body) {
    throw new Error("无法读取事件流");
  }
  const reader = response.body.getReader();
  const decoder = new TextDecoder();
  let buffer = "";
  while (true) {
    const { done, value } = await reader.read();
    if (done) {
      break;
    }
    buffer += decoder.decode(value, { stream: true });
    const frames = readSSEFrames(buffer);
    buffer = frames.rest;
    frames.events.forEach(onEvent);
  }
}
