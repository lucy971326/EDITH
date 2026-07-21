import type { NextRequest } from "next/server";

export const runtime = "nodejs";
export const dynamic = "force-dynamic";

export async function GET(req: NextRequest) {
  const url = new URL(req.url);
  const upstream = await fetch(
    `http://127.0.0.1:8080/sessions?user_id=${url.searchParams.get("user_id") || ""}`,
  );
  return new Response(upstream.body, {
    headers: { "Content-Type": "application/json" },
  });
}
