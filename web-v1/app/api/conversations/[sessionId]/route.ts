import { auth } from "@clerk/nextjs/server";

const runtimeURL = process.env.EDITH_RUNTIME_URL ?? "http://127.0.0.1:8080";

export async function GET(_request: Request, context: RouteContext<"/api/conversations/[sessionId]">) {
  const { userId } = await auth();
  if (!userId) return Response.json({ error: "Unauthorized" }, { status: 401 });
  const { sessionId } = await context.params;
  const url = new URL(`/internal/conversations/${encodeURIComponent(sessionId)}`, runtimeURL);
  url.searchParams.set("userId", userId);
  try {
    const response = await fetch(url, { cache: "no-store" });
    return new Response(response.body, { status: response.status, headers: { "Content-Type": response.headers.get("Content-Type") ?? "application/json", "Cache-Control": "no-store" } });
  } catch { return Response.json({ error: "EDITH runtime is unavailable" }, { status: 502 }); }
}
