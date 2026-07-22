import type { NextRequest } from "next/server";

export const runtime = "nodejs";
export const dynamic = "force-dynamic";

const upstreamURL = "http://127.0.0.1:8080/telegram/configure";

export async function GET(req: NextRequest) {
  const url = new URL(req.url);
  const userID = url.searchParams.get("user_id") ?? "";
  const upstream = await fetch(
    `${upstreamURL}?user_id=${encodeURIComponent(userID)}`,
  );
  return new Response(upstream.body, {
    status: upstream.status,
    headers: { "Content-Type": "application/json" },
  });
}

export async function POST(req: NextRequest) {
  const upstream = await fetch(upstreamURL, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: await req.text(),
  });
  return new Response(upstream.body, {
    status: upstream.status,
    headers: { "Content-Type": "application/json" },
  });
}

export async function DELETE(req: NextRequest) {
  const url = new URL(req.url);
  const userID = url.searchParams.get("user_id") ?? "";
  const upstream = await fetch(
    `${upstreamURL}?user_id=${encodeURIComponent(userID)}`,
    { method: "DELETE" },
  );
  return new Response(upstream.body, {
    status: upstream.status,
    headers: { "Content-Type": "application/json" },
  });
}
