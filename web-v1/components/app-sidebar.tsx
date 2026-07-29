"use client";

import type { ReactNode } from "react";
import { useState } from "react";

type AppSidebarProps = {
  children: ReactNode;
  footer: ReactNode;
};

export function AppSidebar({ children, footer }: AppSidebarProps) {
  const [collapsed, setCollapsed] = useState(false);

  function toggle() {
    setCollapsed((current) => !current);
  }

  return (
    <aside className={`sticky top-0 hidden h-screen shrink-0 flex-col overflow-hidden border-r border-zinc-200 bg-white transition-[width] duration-200 md:flex ${
      collapsed ? "w-14" : "w-64"
    }`}>
      <div className={`flex h-16 shrink-0 items-center border-b border-zinc-200 text-sm font-semibold tracking-wide ${
        collapsed ? "justify-center" : "px-5"
      }`}>
        {collapsed ? (
          <button
            aria-expanded="false"
            aria-label="展开侧边栏"
            className="flex h-8 w-8 items-center justify-center rounded-lg text-lg font-normal text-zinc-500 transition-colors hover:bg-zinc-100 hover:text-zinc-900"
            onClick={toggle}
            title="展开侧边栏"
            type="button"
          >
            ›
          </button>
        ) : (
          <>
            EDITH
            <button
              aria-expanded="true"
              aria-label="折叠侧边栏"
              className="ml-auto flex h-8 w-8 items-center justify-center rounded-lg text-lg font-normal text-zinc-500 transition-colors hover:bg-zinc-100 hover:text-zinc-900"
              onClick={toggle}
              title="折叠侧边栏"
              type="button"
            >
              ‹
            </button>
          </>
        )}
      </div>

      {!collapsed && <>
        {children}
        <div className="border-t border-zinc-200 p-3">{footer}</div>
      </>}
    </aside>
  );
}
