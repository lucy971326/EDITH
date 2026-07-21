import type { NextRequest } from "next/server";

export const runtime = "nodejs";
export const dynamic = "force-dynamic";

export async function POST(req: NextRequest) {
  const upstream = await fetch("http://127.0.0.1:8080/stream", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: await req.text(),
    // 浏览器中断 BFF 请求时，同时中断发往 Go 后端的请求。
    signal: req.signal,
  });

  return new Response(upstream.body, {
    headers: {
      "Content-Type": "text/event-stream",
      "Cache-Control": "no-cache",
      Connection: "keep-alive",
    },
  });
}
