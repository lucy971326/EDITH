import { auth } from "@clerk/nextjs/server";

import type { AgentRunRequest, ChatRequest } from "@/lib/chat/type";

const runtimeURL = process.env.EDITH_RUNTIME_URL ?? "http://127.0.0.1:8080";

// POST /api/chat/stream is EDITH's browser-facing BFF endpoint.
// It intentionally derives userId from Clerk instead of accepting it from JSON.
export async function POST(request: Request) {
  const { userId } = await auth();
  if (!userId) {
    return Response.json({ error: "Unauthorized" }, { status: 401 });
  }

  const chatRequest = await parseChatRequest(request);
  if (!chatRequest) {
    return Response.json(
      { error: "sessionId and message are required strings" },
      { status: 400 },
    );
  }

  const agentRequest: AgentRunRequest = {
    ...chatRequest,
    userId,
  };

  let response: Response;
  try {
    response = await fetch(`${runtimeURL}/internal/agent-runs`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(agentRequest),
      cache: "no-store",
    });
  } catch {
    return Response.json(
      { error: "EDITH runtime is unavailable" },
      { status: 502 },
    );
  }

  return new Response(response.body, {
    status: response.status,
    headers: {
      "Content-Type":
        response.headers.get("Content-Type") ?? "text/event-stream",
      "Cache-Control": "no-cache, no-transform",
      "X-Accel-Buffering": "no",
    },
  });
}

async function parseChatRequest(request: Request): Promise<ChatRequest | null> {
  let value: unknown;
  try {
    value = await request.json();
  } catch {
    return null;
  }

  if (
    typeof value !== "object" ||
    value === null ||
    !("sessionId" in value) ||
    !("message" in value) ||
    !("modelId" in value) ||
    typeof value.sessionId !== "string" ||
    typeof value.message !== "string" ||
    typeof value.modelId !== "string" ||
    !value.sessionId ||
    !value.message.trim() ||
    !value.modelId.trim()
  ) {
    return null;
  }

  return {
    sessionId: value.sessionId,
    message: value.message.trim(),
    modelId: value.modelId.trim(),
  };
}
