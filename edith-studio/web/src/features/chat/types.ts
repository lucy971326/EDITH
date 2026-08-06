import type { AssistantBlock } from "../../lib/stream";

export type UserMessage = { id: string; role: "user"; content: string };
export type AssistantMessage = { id: string; role: "assistant"; blocks: AssistantBlock[] };
export type ChatMessage = UserMessage | AssistantMessage;
