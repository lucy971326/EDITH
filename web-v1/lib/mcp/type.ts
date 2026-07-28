// Browser-facing MCP configuration contract.
// Header values are write-only and are never included in MCPServer responses.

export type MCPTransport = "streamable" | "sse";

export type MCPHeaderState = {
  name: string;
  hasValue: boolean;
};

export type MCPServer = {
  id: string;
  name: string;
  url: string;
  transport: MCPTransport;
  enabled: boolean;
  headers: MCPHeaderState[];
};

export type MCPServerListResponse = {
  servers: MCPServer[];
};

export type CreateMCPHeaderInput = {
  name: string;
  value: string;
};

export type UpdateMCPHeaderInput = {
  name: string;
  value?: string;
};

export type CreateMCPServerRequest = {
  name: string;
  url: string;
  transport: MCPTransport;
  enabled: boolean;
  headers: CreateMCPHeaderInput[];
};

export type UpdateMCPServerRequest = {
  name: string;
  url: string;
  transport: MCPTransport;
  enabled: boolean;
  headers: UpdateMCPHeaderInput[];
};
