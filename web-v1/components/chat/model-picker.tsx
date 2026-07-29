"use client";

import { useEffect, useRef, useState } from "react";

import type { ModelInfo } from "@/lib/models/type";

type ModelPickerProps = {
  disabled: boolean;
  modelID: string;
  models: ModelInfo[];
  onChange: (modelID: string) => void;
};

export function ModelPicker({ disabled, modelID, models, onChange }: ModelPickerProps) {
  const [open, setOpen] = useState(false);
  const picker = useRef<HTMLDivElement>(null);
  const selectedModel = models.find((model) => model.id === modelID);

  useEffect(() => {
    function closeOnOutsideClick(event: MouseEvent) {
      if (!picker.current?.contains(event.target as Node)) {
        setOpen(false);
      }
    }

    function closeOnEscape(event: KeyboardEvent) {
      if (event.key === "Escape") {
        setOpen(false);
      }
    }

    document.addEventListener("mousedown", closeOnOutsideClick);
    document.addEventListener("keydown", closeOnEscape);
    return () => {
      document.removeEventListener("mousedown", closeOnOutsideClick);
      document.removeEventListener("keydown", closeOnEscape);
    };
  }, []);

  function selectModel(nextModelID: string) {
    onChange(nextModelID);
    setOpen(false);
  }

  if (models.length === 0) {
    return <span className="px-2 text-sm text-zinc-400">先在设置配置 API Key</span>;
  }

  return (
    <div className="relative" ref={picker}>
      <button
        aria-expanded={open}
        aria-haspopup="listbox"
        className="flex h-8 max-w-64 items-center gap-2 rounded-lg border border-transparent px-2.5 text-sm font-medium text-zinc-700 transition-colors hover:border-zinc-200 hover:bg-zinc-50 disabled:cursor-not-allowed disabled:text-zinc-400"
        disabled={disabled}
        onClick={() => setOpen((current) => !current)}
        type="button"
      >
        <span className="truncate">{selectedModel?.name ?? "选择模型"}</span>
        {selectedModel?.capabilities.vision && <VisionMark />}
        <span aria-hidden="true" className="text-zinc-400">⌄</span>
      </button>

      {open && <div className="absolute bottom-10 left-0 z-20 w-80 overflow-hidden rounded-xl border border-zinc-200 bg-white shadow-xl shadow-zinc-900/10" role="listbox">
        <div className="border-b border-zinc-100 px-3 py-2.5">
          <p className="text-sm font-medium text-zinc-900">选择模型</p>
          <p className="mt-0.5 text-xs text-zinc-500">仅显示已配置 API Key 的模型</p>
        </div>
        <div className="max-h-72 overflow-y-auto p-1.5">
          {models.map((model) => {
            const selected = model.id === modelID;
            return <button
              aria-selected={selected}
              className={`flex w-full items-center gap-3 rounded-lg border px-3 py-2.5 text-left text-sm transition-colors ${selected ? "border-zinc-200 bg-zinc-100 text-zinc-950" : "border-transparent text-zinc-700 hover:bg-zinc-50"}`}
              key={model.id}
              onClick={() => selectModel(model.id)}
              role="option"
              type="button"
            >
              <span className="min-w-0 flex-1 truncate font-medium">{model.name}</span>
              {model.capabilities.vision && <VisionMark />}
              {selected && <span aria-label="已选择" className="text-zinc-500">✓</span>}
            </button>;
          })}
        </div>
      </div>}
    </div>
  );
}

function VisionMark() {
  return <span className="inline-flex shrink-0 items-center gap-1.5 rounded-full bg-emerald-50 px-2 py-1 text-xs font-medium text-emerald-700">
    <span aria-hidden="true" className="h-1.5 w-1.5 rounded-full bg-emerald-500" />
    Vision
  </span>;
}
