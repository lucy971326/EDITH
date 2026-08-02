"use client";

// MCPServerEditor 负责编辑一个 MCP 服务及其请求头。
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
    <section className="rounded-xl border border-zinc-300 bg-white p-5">
      <div className="flex items-center justify-between">
        <h3 className="text-sm font-medium">{draft.id ? "编辑 MCP 服务" : "添加 MCP 服务"}</h3>
        <label className="flex items-center gap-2 text-sm text-zinc-600">
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
          className="mt-2 h-10 w-full rounded-lg border border-zinc-300 px-3 text-sm outline-none focus:border-zinc-500"
          placeholder="例如 GitHub MCP"
          value={draft.name}
          onChange={(event) => onChange({ ...draft, name: event.target.value })}
        />
      </label>

      <div className="mt-5 grid gap-4 sm:grid-cols-3">
        <label className="block text-sm font-medium sm:col-span-1">
          传输
          <select
            className="mt-2 h-10 w-full rounded-lg border border-zinc-300 bg-white px-3 text-sm outline-none focus:border-zinc-500"
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
            className="mt-2 h-10 w-full rounded-lg border border-zinc-300 px-3 text-sm outline-none focus:border-zinc-500"
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
            <p className="mt-1 text-xs text-zinc-500">可选。密钥只会写入服务端，读取时不会显示。</p>
          </div>
          <button
            className="text-sm text-zinc-600 hover:text-zinc-900"
            type="button"
            onClick={() => onChange({
              ...draft,
              headers: [...draft.headers, { name: "", value: "", hasValue: false }],
            })}
          >
            + 添加 Header
          </button>
        </div>

        <div className="mt-3 space-y-2">
          {draft.headers.map((header, index) => (
            <div className="grid grid-cols-[minmax(0,1fr)_minmax(0,1fr)_2rem] gap-2" key={`${header.name}-${index}`}>
              <input
                className="h-10 min-w-0 rounded-lg border border-zinc-300 px-3 text-sm outline-none focus:border-zinc-500"
                placeholder="Authorization"
                value={header.name}
                onChange={(event) => updateHeader(index, { name: event.target.value })}
              />
              <input
                className="h-10 min-w-0 rounded-lg border border-zinc-300 px-3 text-sm outline-none focus:border-zinc-500"
                placeholder={header.hasValue ? "已配置；留空则不修改" : "输入 Header 值"}
                type="password"
                value={header.value}
                onChange={(event) => updateHeader(index, { value: event.target.value })}
              />
              <button
                aria-label="删除 Header"
                className="rounded-lg text-zinc-500 hover:bg-zinc-100 hover:text-zinc-900"
                type="button"
                onClick={() => onChange({ ...draft, headers: draft.headers.filter((_, current) => current !== index) })}
              >
                ×
              </button>
            </div>
          ))}
        </div>
      </div>

      <div className="mt-5 flex items-center justify-between gap-4">
        <div>
          {onDelete && <button className="text-sm text-red-600 hover:text-red-700" disabled={saving} type="button" onClick={onDelete}>删除</button>}
        </div>
        <div className="flex items-center gap-3">
          <p className="text-sm text-zinc-500">{message}</p>
          <button className="text-sm text-zinc-600 hover:text-zinc-900" disabled={saving} type="button" onClick={onCancel}>取消</button>
          <button className="h-10 rounded-lg bg-zinc-900 px-4 text-sm font-medium text-white hover:bg-zinc-700 disabled:bg-zinc-300" disabled={saving} type="button" onClick={onSave}>{saving ? "保存中…" : "保存"}</button>
        </div>
      </div>
    </section>
  );
}
