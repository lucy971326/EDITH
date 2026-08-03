import { ClerkProvider } from "@clerk/nextjs";
import type { Metadata } from "next";
import "@fontsource-variable/jetbrains-mono/wght.css";
import "@fontsource-variable/noto-sans-sc/wght.css";

import { ThemeProvider } from "@/components/theme-provider";

import "./globals.css";

export const metadata: Metadata = {
  title: "EDITH",
  description: "EDITH Agent Platform",
  icons: {
    icon: "/icon.svg",
  },
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html
      lang="zh-CN"
      className="h-full antialiased"
      suppressHydrationWarning
    >
      <body className="min-h-full bg-canvas text-ink">
        <ClerkProvider>
          <ThemeProvider>{children}</ThemeProvider>
        </ClerkProvider>
      </body>
    </html>
  );
}
