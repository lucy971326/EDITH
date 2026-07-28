import Link from "next/link";
import type { ReactNode } from "react";

type AppSidebarProps = {
  activePage: "chat" | "settings";
  children: ReactNode;
};

export function AppSidebar({ activePage, children }: AppSidebarProps) {
  return (
    <aside className="hidden w-64 shrink-0 flex-col border-r border-zinc-200 bg-white md:flex">
      <div className="flex h-16 items-center border-b border-zinc-200 px-5 text-sm font-semibold tracking-wide">
        EDITH
      </div>

      <nav className="border-b border-zinc-200 p-3">
        <Link
          className={`block rounded-lg px-3 py-2 text-sm transition-colors ${
            activePage === "chat"
              ? "bg-zinc-100 text-zinc-900"
              : "text-zinc-600 hover:bg-zinc-50"
          }`}
          href="/chat"
        >
          对话
        </Link>
        <Link
          className={`mt-1 block rounded-lg px-3 py-2 text-sm transition-colors ${
            activePage === "settings"
              ? "bg-zinc-100 text-zinc-900"
              : "text-zinc-600 hover:bg-zinc-50"
          }`}
          href="/settings"
        >
          设置
        </Link>
      </nav>

      {children}
    </aside>
  );
}
