import type { ReactNode } from "react";

type AppSidebarProps = {
  children: ReactNode;
  footer: ReactNode;
};

export function AppSidebar({ children, footer }: AppSidebarProps) {
  return (
    <aside className="sticky top-0 hidden h-screen w-64 shrink-0 flex-col border-r border-zinc-200 bg-white md:flex">
      <div className="flex h-16 items-center border-b border-zinc-200 px-5 text-sm font-semibold tracking-wide">
        EDITH
      </div>

      {children}
      <div className="border-t border-zinc-200 p-3">{footer}</div>
    </aside>
  );
}
