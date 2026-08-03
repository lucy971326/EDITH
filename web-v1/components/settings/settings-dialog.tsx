"use client";

import { X } from "lucide-react";

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
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/45 p-4 backdrop-blur-sm" role="dialog" aria-modal="true" aria-label="EDITH 设置">
      <section className="max-h-[min(760px,calc(100vh-2rem))] w-full max-w-2xl overflow-y-auto rounded-2xl border border-border bg-canvas shadow-2xl shadow-black/25">
        <header className="flex items-center justify-between border-b border-border bg-surface px-5 py-4">
          <div>
            <h1 className="text-sm font-semibold text-ink">EDITH 设置</h1>
            <p className="mt-0.5 text-xs text-muted">模型凭据与 Agent 配置</p>
          </div>
          <button className="ui-icon-button" onClick={onClose} type="button" aria-label="关闭设置"><X className="size-4" /></button>
        </header>
        <SettingsForm />
      </section>
    </div>
  );
}
