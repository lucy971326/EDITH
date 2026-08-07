import type { AssistantBlock } from "../../lib/stream";

export type ChatImage = { dataUrl: string; name?: string };
export type UserMessage = { id: string; role: "user"; content: string; images?: ChatImage[] };
export type AssistantMessage = { id: string; role: "assistant"; blocks: AssistantBlock[] };
export type ChatMessage = UserMessage | AssistantMessage;
