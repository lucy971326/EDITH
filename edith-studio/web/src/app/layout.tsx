import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = {
  title: "EDITH Studio",
  description: "A local AI Agent for your project.",
};

export default function RootLayout({ children }: LayoutProps<"/">) {
  return (
    <html lang="zh-CN" className="h-full">
      <body className="min-h-full flex flex-col">{children}</body>
    </html>
  );
}
