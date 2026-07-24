import { auth } from "@clerk/nextjs/server";
import type { NextRequest } from "next/server";

export const runtime = "nodejs";
export const dynamic = "force-dynamic";

// 用户文件经 BFF 注入可信 user_id，再交由 Go 写进对应会话的工作区。
export async function POST(req: NextRequest) {
  const { userId } = await auth();
  if (!userId) {
    return Response.json({ error: "Unauthorized" }, { status: 401 });
  }

  let form: FormData;
  try {
    form = await req.formData();
  } catch {
    return Response.json({ error: "Invalid multipart form" }, { status: 400 });
  }

  const sessionID = form.get("session_id");
  const file = form.get("file");
  if (typeof sessionID !== "string" || !(file instanceof File)) {
    return Response.json({ error: "session_id and file are required" }, { status: 400 });
  }

  const upstreamForm = new FormData();
  upstreamForm.set("user_id", userId);
  upstreamForm.set("session_id", sessionID);
  upstreamForm.set("file", file, file.name);

  const upstream = await fetch("http://127.0.0.1:8080/uploads", {
    method: "POST",
    body: upstreamForm,
    signal: req.signal,
  });

  return new Response(upstream.body, {
    status: upstream.status,
    headers: { "Content-Type": upstream.headers.get("Content-Type") || "application/json" },
  });
}
