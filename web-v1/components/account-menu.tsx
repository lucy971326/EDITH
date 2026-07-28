"use client";

import { UserButton } from "@clerk/nextjs";

type AccountMenuProps = {
  onOpenSettings: () => void;
};

// Clerk owns account actions. EDITH only adds its own settings entry.
export function AccountMenu({ onOpenSettings }: AccountMenuProps) {
  return (
    <div className="flex items-center justify-between rounded-xl px-2 py-1 hover:bg-zinc-50">
      <UserButton showName userProfileMode="modal" />
      <button
        aria-label="EDITH 设置"
        className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg text-lg text-zinc-500 transition-colors hover:bg-zinc-200 hover:text-zinc-900"
        onClick={onOpenSettings}
        title="EDITH 设置"
        type="button"
      >
        ⚙
      </button>
    </div>
  );
}
