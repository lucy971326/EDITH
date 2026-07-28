"use client";

import { SettingsForm } from "./settings-form";

type SettingsDialogProps = {
  open: boolean;
  onClose: () => void;
};

export function SettingsDialog({ open, onClose }: SettingsDialogProps) {
  if (!open) {
    return null;
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-zinc-950/20 p-4" role="dialog" aria-modal="true" aria-label="EDITH 设置">
      <section className="max-h-[min(760px,calc(100vh-2rem))] w-full max-w-2xl overflow-y-auto rounded-2xl border border-zinc-200 bg-zinc-50 shadow-xl">
        <header className="flex items-center justify-between border-b border-zinc-200 bg-white px-5 py-4">
          <div>
            <h1 className="text-sm font-semibold">EDITH 设置</h1>
            <p className="mt-0.5 text-xs text-zinc-500">模型凭据与 Agent 配置</p>
          </div>
          <button className="flex h-8 w-8 items-center justify-center rounded-lg text-zinc-500 hover:bg-zinc-100 hover:text-zinc-900" onClick={onClose} type="button" aria-label="关闭设置">×</button>
        </header>
        <SettingsForm />
      </section>
    </div>
  );
}
