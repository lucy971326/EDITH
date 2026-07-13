// Next.js 16 + Clerk v7:
// `clerkMiddleware` + `createRouteMatcher` 已被官方标记 deprecated。
// 阶段 1 页面鉴权靠 page.tsx 里的 `await auth()`；这里只保留 API 路由的鉴权。
import { clerkMiddleware } from "@clerk/nextjs/server";

export default clerkMiddleware(async (auth, request) => {
  // 仅保护 /api/* 下的业务路由；/api/webhook/github 留给后端自身验签
  if (request.nextUrl.pathname.startsWith("/api/")) {
    // /api/webhook/* 是 GitHub → Go 的入口，不需要 Clerk 登录态
    if (request.nextUrl.pathname.startsWith("/api/webhook/")) {
      return;
    }
    await auth.protect();
  }
});

export const config = {
  matcher: [
    // 排除 _next 静态资源、/_next/image 优化、favicon 等
    "/((?!_next|[^?]*\\.(?:html?|css|js(?!on)|jpe?g|webp|png|gif|svg|ttf|woff2?|ico|csv|docx?|xlsx?|zip|webmanifest)).*)",
    // API/TRPC 路由
    "/(api|trpc)(.*)",
  ],
};