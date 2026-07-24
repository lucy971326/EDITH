import { auth } from "@clerk/nextjs/server";
import type { NextRequest } from "next/server";

export const runtime = "nodejs";
export const dynamic = "force-dynamic";

const upstreamURL = "http://127.0.0.1:8080/telegram/configure";

async function authenticatedUserID(): Promise<string | null> {
  const { userId } = await auth();
  return userId;
}

export async function GET() {
  const userID = await authenticatedUserID();
  if (!userID) {
    return Response.json({ error: "Unauthorized" }, { status: 401 });
  }

  const upstream = await fetch(
    `${upstreamURL}?user_id=${encodeURIComponent(userID)}`,
  );
  return new Response(upstream.body, {
    status: upstream.status,
    headers: { "Content-Type": "application/json" },
  });
}

export async function POST(req: NextRequest) {
  const userID = await authenticatedUserID();
  if (!userID) {
    return Response.json({ error: "Unauthorized" }, { status: 401 });
  }

  let input: { bot_token?: unknown };
  try {
    input = await req.json() as { bot_token?: unknown };
  } catch {
    return Response.json({ error: "Invalid JSON" }, { status: 400 });
  }
  if (typeof input.bot_token !== "string" || !input.bot_token.trim()) {
    return Response.json({ error: "bot_token is required" }, { status: 400 });
  }

  const upstream = await fetch(upstreamURL, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      user_id: userID,
      bot_token: input.bot_token.trim(),
    }),
  });
  return new Response(upstream.body, {
    status: upstream.status,
    headers: { "Content-Type": "application/json" },
  });
}

export async function DELETE() {
  const userID = await authenticatedUserID();
  if (!userID) {
    return Response.json({ error: "Unauthorized" }, { status: 401 });
  }

  const upstream = await fetch(
    `${upstreamURL}?user_id=${encodeURIComponent(userID)}`,
    { method: "DELETE" },
  );
  return new Response(upstream.body, {
    status: upstream.status,
    headers: { "Content-Type": "application/json" },
  });
}
