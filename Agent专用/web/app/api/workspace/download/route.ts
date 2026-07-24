import { auth } from "@clerk/nextjs/server";
import type { NextRequest } from "next/server";

export const runtime = "nodejs";
export const dynamic = "force-dynamic";

// 文件下载仍由 BFF 注入 user_id，Go 后端据此只读取该用户该会话的工作区。
export async function GET(req: NextRequest) {
  const { userId } = await auth();
  if (!userId) {
    return Response.json({ error: "Unauthorized" }, { status: 401 });
  }

  const sessionID = req.nextUrl.searchParams.get("session_id");
  const path = req.nextUrl.searchParams.get("path");
  if (!sessionID || !path) {
    return Response.json({ error: "session_id and path are required" }, { status: 400 });
  }

  const query = new URLSearchParams({ user_id: userId, session_id: sessionID, path });
  const upstream = await fetch(`http://127.0.0.1:8080/workspace/download?${query}`);
  const headers = new Headers();
  for (const name of ["Content-Type", "Content-Disposition", "Content-Length"]) {
    const value = upstream.headers.get(name);
    if (value) headers.set(name, value);
  }
  return new Response(upstream.body, { status: upstream.status, headers });
}
