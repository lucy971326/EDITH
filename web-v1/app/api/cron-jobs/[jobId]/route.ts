import { auth } from "@clerk/nextjs/server";

import { parseCronJobRequest } from "@/lib/cron/request";

const runtimeURL = process.env.EDITH_RUNTIME_URL ?? "http://127.0.0.1:8080";

export async function PUT(request: Request, context: RouteContext<"/api/cron-jobs/[jobId]">) {
  const { userId } = await auth();
  if (!userId) return Response.json({ error: "Unauthorized" }, { status: 401 });

  const body = await parseCronJobRequestBody(request);
  if (!body) return Response.json({ error: "Invalid cron job" }, { status: 400 });
  const { jobId } = await context.params;
  return forward(`${runtimeURL}/internal/cron-jobs/${encodeURIComponent(jobId)}`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ name: body.name, taskType: body.taskType, schedule: body.schedule, prompt: body.prompt, userId }),
  });
}

export async function DELETE(_request: Request, context: RouteContext<"/api/cron-jobs/[jobId]">) {
  const { userId } = await auth();
  if (!userId) return Response.json({ error: "Unauthorized" }, { status: 401 });

  const { jobId } = await context.params;
  const url = new URL(`/internal/cron-jobs/${encodeURIComponent(jobId)}`, runtimeURL);
  url.searchParams.set("userId", userId);
  return forward(url, { method: "DELETE" });
}

export async function POST(request: Request, context: RouteContext<"/api/cron-jobs/[jobId]">) {
  const { userId } = await auth();
  if (!userId) return Response.json({ error: "Unauthorized" }, { status: 401 });

  const { jobId } = await context.params;
  const body = await request.json().catch(() => null);
  const enabled = body !== null && typeof body === "object" && typeof (body as { enabled?: unknown }).enabled === "boolean" ? (body as { enabled: boolean }).enabled : false;
  const url = new URL(`/internal/cron-jobs/${encodeURIComponent(jobId)}/toggle`, runtimeURL);
  url.searchParams.set("userId", userId);
  url.searchParams.set("enabled", String(enabled));
  return forward(url, { method: "POST" });
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