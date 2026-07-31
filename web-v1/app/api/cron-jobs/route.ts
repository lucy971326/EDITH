import { auth } from "@clerk/nextjs/server";

import { parseCronJobRequest } from "@/lib/cron/request";

const runtimeURL = process.env.EDITH_RUNTIME_URL ?? "http://127.0.0.1:8080";

export async function GET() {
  const { userId } = await auth();
  if (!userId) return Response.json({ error: "Unauthorized" }, { status: 401 });

  const url = new URL("/internal/cron-jobs", runtimeURL);
  url.searchParams.set("userId", userId);
  return forward(url, { method: "GET" });
}

export async function POST(request: Request) {
  const { userId } = await auth();
  if (!userId) return Response.json({ error: "Unauthorized" }, { status: 401 });

  const body = await parseCronJobRequestBody(request);
  if (!body) return Response.json({ error: "Invalid cron job" }, { status: 400 });
  return forward(`${runtimeURL}/internal/cron-jobs`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ ...body, userId }),
  });
}

async function parseCronJobRequestBody(request: Request) {
  try {
    return parseCronJobRequest(await request.json());
  } catch {
    return null;
  }
}

async function forward(url: string | URL, init: RequestInit) {
  try {
    const response = await fetch(url, { ...init, cache: "no-store" });
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