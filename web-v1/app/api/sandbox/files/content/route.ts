import { auth } from "@clerk/nextjs/server";

const runtimeURL = process.env.EDITH_RUNTIME_URL ?? "http://127.0.0.1:8080";

export async function GET(request: Request) {
  const { userId } = await auth();
  if (!userId) return Response.json({ error: "Unauthorized" }, { status: 401 });

  const requestURL = new URL(request.url);
  const sessionId = requestURL.searchParams.get("sessionId");
  const path = requestURL.searchParams.get("path");
  if (!sessionId || !path) return Response.json({ error: "sessionId and path are required" }, { status: 400 });

  const url = new URL("/internal/sandbox/files/content", runtimeURL);
  url.searchParams.set("userId", userId);
  url.searchParams.set("sessionId", sessionId);
  url.searchParams.set("path", path);
  try {
    const response = await fetch(url, { cache: "no-store" });
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
