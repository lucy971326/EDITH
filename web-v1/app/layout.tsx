import { ClerkProvider } from "@clerk/nextjs";
import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = {
  title: "EDITH",
  description: "EDITH Agent Platform",
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
    >
      <body className="min-h-full bg-zinc-50 text-zinc-900">
        <ClerkProvider>
          {children}
        </ClerkProvider>
      </body>
    </html>
  );
}
