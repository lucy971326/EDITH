"use client";

import { UserButton } from "@clerk/nextjs";
import { Settings } from "lucide-react";

import { ThemeToggle } from "./theme-toggle";

type AccountMenuProps = {
  onOpenSettings: () => void;
};

// Clerk owns account actions. EDITH only adds its own settings entry.
export function AccountMenu({ onOpenSettings }: AccountMenuProps) {
  return (
    <div className="flex items-center justify-between rounded-xl px-2 py-1 hover:bg-surface-subtle">
      <UserButton showName userProfileMode="modal" />
      <div className="flex items-center gap-1">
        <ThemeToggle />
        <button aria-label="EDITH 设置" className="ui-icon-button shrink-0" onClick={onOpenSettings} title="EDITH 设置" type="button"><Settings className="size-4" /></button>
      </div>
    </div>
  );
}
