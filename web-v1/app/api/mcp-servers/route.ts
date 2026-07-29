import { auth } from "@clerk/nextjs/server";

import { parseMCPServerRequest } from "@/lib/mcp/request";

const runtimeURL = process.env.EDITH_RUNTIME_URL ?? "http://127.0.0.1:8080";

export async function GET() {
  const { userId } = await auth();
  if (!userId) return Response.json({ error: "Unauthorized" }, { status: 401 });

  const url = new URL("/internal/mcp-servers", runtimeURL);
  url.searchParams.set("userId", userId);
  return forward(url, { method: "GET" });
}

export async function POST(request: Request) {
  const { userId } = await auth();
  if (!userId) return Response.json({ error: "Unauthorized" }, { status: 401 });

  const body = await parseMCPServerRequestBody(request);
  if (!body) return Response.json({ error: "Invalid MCP server" }, { status: 400 });
  return forward(`${runtimeURL}/internal/mcp-servers`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ ...body, userId }),
  });
}

async function parseMCPServerRequestBody(request: Request) {
  try {
    return parseMCPServerRequest(await request.json());
  } catch {
    return null;
  }
}

async function forward(url: string | URL, init: RequestInit) {
  try {
    const response = await fetch(url, { ...init, cache: "no-store" });
    return new Response(response.body, {
      status: response.status,
      headers: {
        "Content-Type": response.headers.get("Content-Type") ?? "application/json",
        "Cache-Control": "no-store",
      },
    });
  } catch {
    return Response.json({ error: "EDITH runtime is unavailable" }, { status: 502 });
  }
}
