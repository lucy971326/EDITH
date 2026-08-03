import { auth } from "@clerk/nextjs/server";

const runtimeURL = process.env.EDITH_RUNTIME_URL ?? "http://127.0.0.1:8080";

// POST 透传浏览器 multipart 流，避免 BFF 将 50MB 文件读入内存。
export async function POST(request: Request) {
  const { userId } = await auth();
  if (!userId) return Response.json({ error: "Unauthorized" }, { status: 401 });
  const sessionId = new URL(request.url).searchParams.get("sessionId");
  if (!sessionId) return Response.json({ error: "sessionId is required" }, { status: 400 });
  const url = new URL("/internal/sandbox/files/upload", runtimeURL);
  url.searchParams.set("userId", userId); url.searchParams.set("sessionId", sessionId);
  try {
    const init: RequestInit & { duplex: "half" } = { method: "POST", headers: { "Content-Type": request.headers.get("Content-Type") ?? "" }, body: request.body, duplex: "half", cache: "no-store" };
    const response = await fetch(url, init);
    return new Response(response.body, { status: response.status, headers: { "Content-Type": response.headers.get("Content-Type") ?? "application/json", "Cache-Control": "no-store" } });
  } catch { return Response.json({ error: "EDITH runtime is unavailable" }, { status: 502 }); }
}
