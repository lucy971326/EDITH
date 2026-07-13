import { auth } from "@clerk/nextjs/server";
import { NextResponse } from "next/server";

// GET /api/me
// 阶段 1：返回当前 Clerk 用户身份（来自 Clerk session cookie + JWT）
// 阶段 2：会改成转发给 Go 后端 /api/me，验证 Go 也能解析 Clerk Bearer Token
export async function GET() {
  const { userId, sessionId, sessionClaims } = await auth();

  if (!userId) {
    return NextResponse.json({ error: "unauthorized" }, { status: 401 });
  }

  return NextResponse.json({
    clerk_user_id: userId,
    session_id: sessionId,
    // Clerk v7 的 sessionClaims 是个对象；这里只取几个常见字段
    claims: {
      email:
        (sessionClaims as { email?: string } | null)?.email ?? null,
    },
  });
}