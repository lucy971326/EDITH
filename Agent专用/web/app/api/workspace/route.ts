import { auth } from "@clerk/nextjs/server";
import type { NextRequest } from "next/server";

export const runtime = "nodejs";
export const dynamic = "force-dynamic";

// BFF 从登录态取得 user_id，浏览器只能指定要浏览的 session 与相对路径。
export async function GET(req: NextRequest) {
  const { userId } = await auth();
  if (!userId) {
    return Response.json({ error: "Unauthorized" }, { status: 401 });
  }

  const sessionID = req.nextUrl.searchParams.get("session_id");
  const path = req.nextUrl.searchParams.get("path") || ".";
  if (!sessionID) {
    return Response.json({ error: "session_id is required" }, { status: 400 });
  }

  const query = new URLSearchParams({ user_id: userId, session_id: sessionID, path });
  const upstream = await fetch(`http://127.0.0.1:8080/workspace?${query}`);
  return new Response(upstream.body, {
    status: upstream.status,
    headers: { "Content-Type": upstream.headers.get("Content-Type") || "application/json" },
  });
}
