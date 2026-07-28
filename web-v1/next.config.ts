import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  // Hide Next.js' development-only floating Dev Tools button.
  devIndicators: false,
  turbopack: {
    root: process.cwd(),
  },
};

export default nextConfig;
