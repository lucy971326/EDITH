import { auth } from "@clerk/nextjs/server";
const runtimeURL = process.env.EDITH_RUNTIME_URL ?? "http://127.0.0.1:8080";
export async function GET(request: Request) {
  const { userId } = await auth(); if (!userId) return Response.json({ error: "Unauthorized" }, { status: 401 });
  const source = new URL(request.url); const sessionId = source.searchParams.get("sessionId"); const path = source.searchParams.get("path");
  if (!sessionId || !path) return Response.json({ error: "sessionId and path are required" }, { status: 400 });
  const url = new URL("/internal/sandbox/files/download", runtimeURL); url.searchParams.set("userId", userId); url.searchParams.set("sessionId", sessionId); url.searchParams.set("path", path);
  try { const response = await fetch(url, { cache: "no-store" }); return new Response(response.body, { status: response.status, headers: { "Content-Type": response.headers.get("Content-Type") ?? "application/octet-stream", "Content-Disposition": response.headers.get("Content-Disposition") ?? "attachment", "Cache-Control": "no-store" } }); }
  catch { return Response.json({ error: "EDITH runtime is unavailable" }, { status: 502 }); }
}
