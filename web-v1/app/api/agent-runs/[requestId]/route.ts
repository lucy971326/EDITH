import { auth } from "@clerk/nextjs/server";

const runtimeURL = process.env.EDITH_RUNTIME_URL ?? "http://127.0.0.1:8080";

async function forward(path: string, userId: string, method = "GET"): Promise<Response> {
  const url = new URL(path, runtimeURL);
  url.searchParams.set("userId", userId);
  try {
    const response = await fetch(url, { method, cache: "no-store" });
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

export async function GET(
  _request: Request,
  { params }: { params: Promise<{ requestId: string }> },
) {
  const { userId } = await auth();
  if (!userId) return Response.json({ error: "Unauthorized" }, { status: 401 });

  const { requestId } = await params;
  return forward(`/internal/agent-runs/${encodeURIComponent(requestId)}`, userId);
}

export async function POST(
  _request: Request,
  { params }: { params: Promise<{ requestId: string }> },
) {
  const { userId } = await auth();
  if (!userId) return Response.json({ error: "Unauthorized" }, { status: 401 });

  const { requestId } = await params;
  return forward(`/internal/agent-runs/${encodeURIComponent(requestId)}/cancel`, userId, "POST");
}
