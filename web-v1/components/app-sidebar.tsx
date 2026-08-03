"use client";

import type { ReactNode } from "react";
import { useState } from "react";
import { PanelLeftClose, PanelLeftOpen } from "lucide-react";

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
    <aside className={`sticky top-0 hidden h-screen shrink-0 flex-col overflow-hidden border-r border-border transition-[width,background-color] duration-200 md:flex ${
      collapsed ? "w-11 bg-canvas" : "w-56 bg-surface"
    }`}>
      <div className={`flex h-14 shrink-0 items-center border-b border-border text-[13px] font-semibold tracking-[0.16em] text-ink ${
        collapsed ? "justify-center" : "px-5"
      }`}>
        {collapsed ? (
          <button
            aria-expanded="false"
            aria-label="展开侧边栏"
            className="ui-icon-button"
            onClick={toggle}
            title="展开侧边栏"
            type="button"
          >
            <PanelLeftOpen className="size-5" />
          </button>
        ) : (
          <>
            <span className="flex items-center gap-2 font-semibold">
              <img alt="EDITH" className="size-7 rounded-lg" src="/icon.svg" />
              EDITH
            </span>
            <button
              aria-expanded="true"
              aria-label="折叠侧边栏"
              className="ui-icon-button ml-auto"
              onClick={toggle}
              title="折叠侧边栏"
              type="button"
            >
              <PanelLeftClose className="size-5" />
            </button>
          </>
        )}
      </div>

      {!collapsed && <>
        {children}
        <div className="border-t border-border p-3">{footer}</div>
      </>}
    </aside>
  );
}
