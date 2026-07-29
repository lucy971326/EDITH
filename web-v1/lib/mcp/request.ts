import type { MCPTransport, UpdateMCPHeaderInput, UpdateMCPServerRequest } from "./type";

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

// parseMCPServerRequest is the browser-facing MCP configuration boundary
// before Clerk's userId is added by the BFF route.
export function parseMCPServerRequest(value: unknown): UpdateMCPServerRequest | null {
  if (
    !isRecord(value) ||
    typeof value.name !== "string" || !value.name.trim() ||
    typeof value.url !== "string" || !value.url.trim() ||
    (value.transport !== "streamable" && value.transport !== "sse") ||
    typeof value.enabled !== "boolean" ||
    !Array.isArray(value.headers)
  ) return null;

  const headers: UpdateMCPHeaderInput[] = [];
  for (const header of value.headers) {
    if (!isRecord(header) || typeof header.name !== "string" || !header.name.trim()) {
      return null;
    }
    if ("value" in header && typeof header.value !== "string") {
      return null;
    }
    headers.push({
      name: header.name.trim(),
      ...(typeof header.value === "string" ? { value: header.value } : {}),
    });
  }

  return {
    name: value.name.trim(),
    url: value.url.trim(),
    transport: value.transport as MCPTransport,
    enabled: value.enabled,
    headers,
  };
}
