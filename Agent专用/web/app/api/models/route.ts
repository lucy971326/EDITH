export const runtime = "nodejs";
export const dynamic = "force-dynamic";

export async function GET() {
  const upstream = await fetch("http://127.0.0.1:8080/models");

  return new Response(upstream.body, {
    headers: { "Content-Type": "application/json" },
  });
}
