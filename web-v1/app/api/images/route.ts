import { auth } from "@clerk/nextjs/server";

import type { CreateImageUploadRequest } from "@/lib/chat/type";

const runtimeURL = process.env.EDITH_RUNTIME_URL ?? "http://127.0.0.1:8080";

export async function POST(request: Request) {
  const { userId } = await auth();
  if (!userId) return Response.json({ error: "Unauthorized" }, { status: 401 });

  const body = await parseCreateImageUploadRequest(request);
  if (!body) return Response.json({ error: "Invalid image upload request" }, { status: 400 });

  try {
    const response = await fetch(`${runtimeURL}/internal/images`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ ...body, userId }),
      cache: "no-store",
    });
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

async function parseCreateImageUploadRequest(request: Request): Promise<CreateImageUploadRequest | null> {
  try {
    const value: unknown = await request.json();
    if (
      typeof value !== "object" || value === null ||
      !("sessionId" in value) || !("mimeType" in value) || !("sizeBytes" in value) ||
      typeof value.sessionId !== "string" || typeof value.mimeType !== "string" ||
      typeof value.sizeBytes !== "number" || !Number.isInteger(value.sizeBytes) ||
      !value.sessionId.trim() || !value.mimeType.trim() || value.sizeBytes <= 0
    ) return null;

    return {
      sessionId: value.sessionId.trim(),
      mimeType: value.mimeType.trim(),
      sizeBytes: value.sizeBytes,
    };
  } catch {
    return null;
  }
}
