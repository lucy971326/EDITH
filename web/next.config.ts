import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  // 隐藏 Next.js 开发环境左下角的调试指示器。
  devIndicators: false,
  // 当前 Next.js 应用本身就是追踪根，避免误选到上层的 package-lock.json。
  outputFileTracingRoot: __dirname,
  // 开发时允许通过 http://127.0.0.1:3000 访问 HMR 资源。
  allowedDevOrigins: ["127.0.0.1"],
};

export default nextConfig;
