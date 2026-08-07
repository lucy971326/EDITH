import { apiJSON } from "./client";

export type McpServerStatus = {
  name: string;
  transport: string;
  status: "connected" | "error";
  toolCount: number;
  error?: string;
};

export type McpStatus = {
  servers: McpServerStatus[];
};

export async function getMcpStatus(): Promise<McpStatus> {
  return apiJSON<McpStatus>("/api/mcp");
}
