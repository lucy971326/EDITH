"use client";

import { PageSidebar } from "@/components/page-sidebar";

import { MCPServers } from "./mcp-servers";
import { Skills } from "./skills";

// ExtensionsPage 展示当前阶段的扩展配置入口。
export function ExtensionsPage() {
  return (
    <main className="flex h-screen overflow-hidden bg-zinc-50">
      <PageSidebar activePage="extensions" />
      <section className="min-w-0 flex-1 overflow-y-auto p-6">
        <header className="mb-6">
          <h1 className="text-lg font-semibold text-zinc-900">扩展</h1>
          <p className="mt-0.5 text-sm text-zinc-500">管理 EDITH 可以使用的外部能力。</p>
        </header>
        <div className="space-y-6">
          <Skills />
          <MCPServers />
        </div>
      </section>
    </main>
  );
}
