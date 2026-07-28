// EDITH's browser-facing conversation contract.
//
// Do not import or model trpc-agent-go events in the Web app. Live SSE events
// and historical session data are both projected into this timeline shape.

export type Timeline = {
  blocks: TimelineBlock[];
};

export type TimelineBlock = UserBlock | AssistantBlock | ErrorBlock;

export type UserBlock = {
  type: "user";
  id: string;
  content: string;
  createdAt: string;
};

export type AssistantBlock = {
  type: "assistant";
  id: string;
  createdAt: string;
  blocks: AssistantContentBlock[];
};

export type ErrorBlock = {
  type: "error";
  id: string;
  message: string;
  createdAt: string;
};

export type AssistantContentBlock =
  | ReasoningBlock
  | TextBlock
  | ToolBlock;

export type ReasoningBlock = {
  type: "reasoning";
  id: string;
  content: string;
};

export type TextBlock = {
  type: "text";
  id: string;
  content: string;
};

export type ToolBlock = {
  type: "tool";
  id: string; // trpc-agent-go ToolCall.ID
  toolName: string;
  arguments: string;
  status: "running" | "completed" | "failed";
  result?: string;
};

// Browser → Next BFF. userId must never be supplied by the browser.
export type ChatRequest = {
  sessionId: string;
  message: string;
  modelId: string;
  reasoningOptionId?: string;
};

export type Conversation = {
  id: string;
  title: string;
  updatedAt: string;
};

export type ConversationListResponse = {
  conversations: Conversation[];
};

export type ConversationResponse = {
  timeline: Timeline;
};

// Next BFF → internal Go Runtime. userId comes from Clerk on the BFF.
export type AgentRunRequest = ChatRequest & {
  userId: string;
};

// The JSON payload carried by one EDITH SSE event.
export type ChatStreamEvent =
  | AssistantStartedEvent
  | ContentDeltaEvent
  | ToolStartedEvent
  | ToolFinishedEvent
  | ErrorEvent
  | DoneEvent;

export type AssistantStartedEvent = {
  type: "assistant.started";
  assistant: AssistantBlock;
};

export type ContentDeltaEvent = {
  type: "assistant.content.delta";
  assistantId: string;
  blockId: string;
  blockType: "reasoning" | "text";
  delta: string;
};

export type ToolStartedEvent = {
  type: "tool.started";
  assistantId: string;
  tool: ToolBlock;
};

export type ToolFinishedEvent = {
  type: "tool.finished";
  assistantId: string;
  toolCallId: string;
  status: "completed" | "failed";
  result?: string;
};

export type ErrorEvent = {
  type: "error";
  error: ErrorBlock;
};

export type DoneEvent = {
  type: "done";
  requestId: string;
  usage?: Usage;
};

export type Usage = {
  promptTokens: number;
  completionTokens: number;
  totalTokens: number;
};
