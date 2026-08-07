import { apiJSON, apiRequest } from "./client";

export type CommandDefinition = {
  name: string;
  description: string;
  syntax: string;
};

export type CommandInput = {
  sessionId: string;
  command: string;
  modelId: string;
  thinkingMode: string;
};

export async function listCommands(): Promise<CommandDefinition[]> {
  const result = await apiJSON<{ commands?: CommandDefinition[] }>("/api/commands");
  return result.commands ?? [];
}

// executeCommand 返回原始 Response；调用方根据 ok 和 message 判断成功或失败。
export function executeCommand(input: CommandInput) {
  return apiRequest("/api/commands", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(input),
  });
}
