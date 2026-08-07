const agentAPI = "http://127.0.0.1:8765";

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
  const response = await fetch(`${agentAPI}/api/commands`);
  if (!response.ok) {
    throw new Error("无法读取命令目录");
  }
  const body = await response.json() as { commands?: CommandDefinition[] };
  return body.commands ?? [];
}

export function executeCommand(input: CommandInput) {
  return fetch(`${agentAPI}/api/commands`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(input),
  });
}
