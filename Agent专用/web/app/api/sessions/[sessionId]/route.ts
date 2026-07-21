import type { NextRequest } from "next/server";

export const runtime = "nodejs";
export const dynamic = "force-dynamic";

export async function GET(
  _req: NextRequest,
  { params }: { params: Promise<{ sessionId: string }> },
) {
  const { sessionId } = await params;
  const url = new URL(_req.url);
  const userID = url.searchParams.get("user_id") || "";

  const upstream = await fetch(
    `http://127.0.0.1:8080/sessions/${sessionId}?user_id=${userID}`,
  );
  return new Response(upstream.body, {
    headers: { "Content-Type": "application/json" },
  });
}
