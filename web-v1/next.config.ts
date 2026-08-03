import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  // Hide Next.js' development-only floating Dev Tools button.
  devIndicators: false,
  // Minimal production image: emit .next/standalone for Docker.
  output: "standalone",
  turbopack: {
    root: process.cwd(),
  },
};

export default nextConfig;
