"use client";

// MCPServerEditor 负责编辑一个 MCP 服务及其请求头。
import { Plus, Trash2, X } from "lucide-react";

import type { MCPTransport } from "@/lib/mcp/type";

export type MCPHeaderDraft = {
  name: string;
  value: string;
  hasValue: boolean;
};

export type MCPServerDraft = {
  id?: string;
  name: string;
  url: string;
  transport: MCPTransport;
  enabled: boolean;
  headers: MCPHeaderDraft[];
};

type MCPServerEditorProps = {
  draft: MCPServerDraft;
  message: string;
  saving: boolean;
  onChange: (draft: MCPServerDraft) => void;
  onCancel: () => void;
  onDelete?: () => void;
  onSave: () => void;
};

export function MCPServerEditor({
  draft,
  message,
  saving,
  onChange,
  onCancel,
  onDelete,
  onSave,
}: MCPServerEditorProps) {
  function updateHeader(index: number, change: Partial<MCPHeaderDraft>) {
    onChange({
      ...draft,
      headers: draft.headers.map((header, current) => current === index ? { ...header, ...change } : header),
    });
  }

  return (
    <section className="rounded-xl border border-border-strong bg-surface p-5">
      <div className="flex items-center justify-between">
        <h3 className="text-sm font-medium text-ink">{draft.id ? "编辑 MCP 服务" : "添加 MCP 服务"}</h3>
        <label className="flex items-center gap-2 text-sm text-muted">
          <input
            checked={draft.enabled}
            type="checkbox"
            onChange={(event) => onChange({ ...draft, enabled: event.target.checked })}
          />
          启用
        </label>
      </div>

      <label className="mt-5 block text-sm font-medium">
        名称
        <input
          className="ui-field mt-2 h-10 w-full px-3"
          placeholder="例如 GitHub MCP"
          value={draft.name}
          onChange={(event) => onChange({ ...draft, name: event.target.value })}
        />
      </label>

      <div className="mt-5 grid gap-4 sm:grid-cols-3">
        <label className="block text-sm font-medium sm:col-span-1">
          传输
          <select
            className="ui-field mt-2 h-10 w-full px-3"
            value={draft.transport}
            onChange={(event) => onChange({ ...draft, transport: event.target.value as MCPTransport })}
          >
            <option value="streamable">Streamable HTTP</option>
            <option value="sse">Legacy SSE</option>
          </select>
        </label>
        <label className="block text-sm font-medium sm:col-span-2">
          地址
          <input
          className="ui-field mt-2 h-10 w-full px-3"
            placeholder="https://example.com/mcp"
            type="url"
            value={draft.url}
            onChange={(event) => onChange({ ...draft, url: event.target.value })}
          />
        </label>
      </div>

      <div className="mt-5">
        <div className="flex items-baseline justify-between gap-4">
          <div>
            <h4 className="text-sm font-medium">HTTP Headers</h4>
            <p className="mt-1 text-xs text-muted">可选。密钥只会写入服务端，读取时不会显示。</p>
          </div>
          <button
            className="inline-flex items-center gap-1 text-sm text-muted transition-colors hover:text-ink"
            type="button"
            onClick={() => onChange({
              ...draft,
              headers: [...draft.headers, { name: "", value: "", hasValue: false }],
            })}
          >
            <Plus className="size-3.5" />添加 Header
          </button>
        </div>

        <div className="mt-3 space-y-2">
          {draft.headers.map((header, index) => (
            <div className="grid grid-cols-[minmax(0,1fr)_minmax(0,1fr)_2rem] gap-2" key={`${header.name}-${index}`}>
              <input
                className="ui-field h-10 min-w-0 px-3"
                placeholder="Authorization"
                value={header.name}
                onChange={(event) => updateHeader(index, { name: event.target.value })}
              />
              <input
                className="ui-field h-10 min-w-0 px-3"
                placeholder={header.hasValue ? "已配置；留空则不修改" : "输入 Header 值"}
                type="password"
                value={header.value}
                onChange={(event) => updateHeader(index, { value: event.target.value })}
              />
              <button
                aria-label="删除 Header"
                className="ui-icon-button"
                type="button"
                onClick={() => onChange({ ...draft, headers: draft.headers.filter((_, current) => current !== index) })}
              >
                <X className="size-4" />
              </button>
            </div>
          ))}
        </div>
      </div>

      <div className="mt-5 flex items-center justify-between gap-4">
        <div>
          {onDelete && <button className="ui-button-danger h-auto border-0 px-0" disabled={saving} type="button" onClick={onDelete}><Trash2 className="size-3.5" />删除</button>}
        </div>
        <div className="flex items-center gap-3">
          <p className="text-sm text-muted">{message}</p>
          <button className="text-sm text-muted hover:text-ink" disabled={saving} type="button" onClick={onCancel}>取消</button>
          <button className="ui-button-primary" disabled={saving} type="button" onClick={onSave}>{saving ? "保存中…" : "保存"}</button>
        </div>
      </div>
    </section>
  );
}
