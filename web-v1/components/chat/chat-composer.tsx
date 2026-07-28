"use client";

import { useState } from "react";

import type { ModelInfo } from "@/lib/models/type";

type ChatComposerProps = {
  isRunning: boolean;
  modelID: string;
  models: ModelInfo[];
  reasoningOptionID: string;
  selectedModel?: ModelInfo;
  onModelChange: (modelID: string) => void;
  onReasoningOptionChange: (reasoningOptionID: string) => void;
  onSend: (message: string) => void;
};

export function ChatComposer({
  isRunning,
  modelID,
  models,
  reasoningOptionID,
  selectedModel,
  onModelChange,
  onReasoningOptionChange,
  onSend,
}: ChatComposerProps) {
  const [message, setMessage] = useState("");

  function send() {
    const content = message.trim();
    if (!content || !modelID || isRunning) {
      return;
    }
    setMessage("");
    onSend(content);
  }

  return (
    <form
      className="border-t border-zinc-200 bg-white p-4"
      onSubmit={(event) => {
        event.preventDefault();
        send();
      }}
    >
      <div className="mx-auto max-w-3xl rounded-2xl border border-zinc-300 bg-white p-3 shadow-sm transition-colors focus-within:border-zinc-400">
        <textarea
          className="block min-h-20 w-full resize-none bg-transparent px-1 py-1 text-sm leading-6 outline-none placeholder:text-zinc-400"
          placeholder="输入消息…"
          rows={2}
          value={message}
          onChange={(event) => setMessage(event.target.value)}
          onKeyDown={(event) => {
            if (event.key !== "Enter" || event.shiftKey) {
              return;
            }
            event.preventDefault();
            send();
          }}
        />
        <div className="mt-2 flex items-center gap-2 border-t border-zinc-100 pt-2">
          <button className="flex h-8 w-8 items-center justify-center rounded-lg text-lg text-zinc-500 transition-colors hover:bg-zinc-100 hover:text-zinc-900" type="button" title="更多输入能力（即将支持）">+</button>
          <select className="h-8 max-w-52 rounded-lg bg-transparent px-2 text-sm font-medium text-zinc-700 outline-none hover:bg-zinc-100" disabled={isRunning || models.length === 0} value={modelID} onChange={(event) => onModelChange(event.target.value)}>
            {models.length === 0 && <option value="">先在设置配置 API Key</option>}
            {models.map((model) => <option key={model.id} value={model.id}>{model.name}</option>)}
          </select>
          {selectedModel && selectedModel.reasoningOptions.length > 0 && <>
            <span className="text-zinc-300">·</span>
            <select className="h-8 rounded-lg bg-transparent px-2 text-sm text-zinc-600 outline-none hover:bg-zinc-100" disabled={isRunning} value={reasoningOptionID} onChange={(event) => onReasoningOptionChange(event.target.value)}>
              <option value="">思考</option>
              {selectedModel.reasoningOptions.map((option) => <option key={option.id} value={option.id}>{option.name}</option>)}
            </select>
          </>}
          <button
            aria-label="发送"
            className="ml-auto flex h-9 w-9 items-center justify-center rounded-xl bg-zinc-900 text-lg text-white transition-colors hover:bg-zinc-700 disabled:cursor-not-allowed disabled:bg-zinc-200 disabled:text-zinc-400"
            disabled={!message.trim() || !modelID || isRunning}
            type="submit"
          >
            {isRunning ? "…" : "↑"}
          </button>
        </div>
      </div>
    </form>
  );
}
