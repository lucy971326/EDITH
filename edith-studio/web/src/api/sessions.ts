import type { ChatMessage } from "../features/chat/types";
import type { SessionSummary } from "../features/sessions/types";

const studioAPI = "http://127.0.0.1:8765";

export type SessionHistory = { session: SessionSummary; messages: ChatMessage[] };

export async function listSessions(): Promise<SessionSummary[]> {
  const response = await fetch(`${studioAPI}/api/sessions`);
  if (!response.ok) throw new Error("无法读取会话列表");
  const result = await response.json() as { sessions: SessionSummary[] };
  return result.sessions;
}

export async function getSession(sessionID: string): Promise<SessionHistory> {
  const response = await fetch(`${studioAPI}/api/sessions/${encodeURIComponent(sessionID)}`);
  if (!response.ok) throw new Error(response.status === 404 ? "会话不存在" : "无法读取会话历史");
  return response.json() as Promise<SessionHistory>;
}

export async function deleteSession(sessionID: string) {
  const response = await fetch(`${studioAPI}/api/sessions/${encodeURIComponent(sessionID)}`, { method: "DELETE" });
  if (!response.ok) throw new Error(response.status === 404 ? "会话不存在" : "无法删除会话");
}
