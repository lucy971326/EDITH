import { auth } from "@clerk/nextjs/server";
import type { NextRequest } from "next/server";

export const runtime = "nodejs";
export const dynamic = "force-dynamic";

export async function POST(req: NextRequest) {
  const { userId } = await auth();
  if (!userId) {
    return Response.json({ error: "Unauthorized" }, { status: 401 });
  }

  let input: Record<string, unknown>;
  try {
    input = await req.json() as Record<string, unknown>;
  } catch {
    return Response.json({ error: "Invalid JSON" }, { status: 400 });
  }

  const upstream = await fetch("http://127.0.0.1:8080/stream", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    // user_id 永远由登录态覆盖，不相信浏览器提交的身份。
    body: JSON.stringify({ ...input, user_id: userId }),
    // 浏览器中断 BFF 请求时，同时中断发往 Go 后端的请求。
    signal: req.signal,
  });

  return new Response(upstream.body, {
    status: upstream.status,
    headers: {
      "Content-Type": "text/event-stream",
      "Cache-Control": "no-cache",
      Connection: "keep-alive",
    },
  });
}
