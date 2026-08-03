"use client";

import { PageSidebar } from "@/components/page-sidebar";

import { MCPServers } from "./mcp-servers";
import { Skills } from "./skills";

// ExtensionsPage 展示当前阶段的扩展配置入口。
export function ExtensionsPage() {
  return (
    <main className="flex h-screen overflow-hidden bg-canvas">
      <PageSidebar activePage="extensions" />
      <section className="min-w-0 flex-1 overflow-y-auto p-6 md:p-8">
        <div className="mx-auto max-w-6xl">
          <header className="mb-8">
            <p className="mb-2 text-xs font-medium tracking-[0.12em] text-accent">WORKSPACE</p>
            <h1 className="ui-page-title text-2xl">扩展</h1>
            <p className="ui-page-description">管理 EDITH 可以调用的知识、工具与外部服务。</p>
          </header>
          <div className="space-y-5">
            <Skills />
            <MCPServers />
          </div>
        </div>
      </section>
    </main>
  );
}
