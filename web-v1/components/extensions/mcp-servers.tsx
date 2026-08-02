"use client";

// MCPServers 负责加载并维护当前用户的 MCP 服务配置。
import { useEffect, useState } from "react";

import type {
  CreateMCPServerRequest,
  MCPServer,
  MCPServerListResponse,
  UpdateMCPServerRequest,
} from "@/lib/mcp/type";

import { MCPServerEditor, type MCPServerDraft } from "./mcp-server-editor";

function newDraft(): MCPServerDraft {
  return { name: "", url: "", transport: "streamable", enabled: true, headers: [] };
}

function draftFromServer(server: MCPServer): MCPServerDraft {
  return {
    ...server,
    headers: server.headers.map((header) => ({ ...header, value: "" })),
  };
}

function validate(draft: MCPServerDraft): string {
  if (!draft.name.trim()) return "请填写名称。";

  let url: URL;
  try {
    url = new URL(draft.url);
  } catch {
    return "请输入有效的 HTTP 地址。";
  }
  if (url.protocol !== "http:" && url.protocol !== "https:") return "仅支持 HTTP 或 HTTPS 地址。";

  const names = new Set<string>();
  for (const header of draft.headers) {
    const name = header.name.trim();
    if (!name) return "Header 名称不能为空。";
    if (names.has(name.toLowerCase())) return "不能重复配置同名 Header。";
    if (!header.hasValue && !header.value.trim()) return `请填写 ${name} 的值。`;
    names.add(name.toLowerCase());
  }
  return "";
}

export function MCPServers() {
  const [servers, setServers] = useState<MCPServer[]>([]);
  const [editing, setEditing] = useState<MCPServerDraft | null>(null);
  const [message, setMessage] = useState("");
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    async function load() {
      const response = await fetch("/api/mcp-servers");
      if (!response.ok) {
        setMessage("无法加载 MCP 服务。");
        return;
      }

      const body = await response.json() as MCPServerListResponse;
      setServers(body.servers);
    }
    void load();
  }, []);

  async function save() {
    if (!editing) return;

    const validationMessage = validate(editing);
    if (validationMessage) {
      setMessage(validationMessage);
      return;
    }

    const isUpdate = Boolean(editing.id);
    const headers = editing.headers.map((header) => {
      const name = header.name.trim();
      const value = header.value.trim();
      return value ? { name, value } : { name };
    });
    const body: CreateMCPServerRequest | UpdateMCPServerRequest = {
      name: editing.name.trim(),
      url: editing.url.trim(),
      transport: editing.transport,
      enabled: editing.enabled,
      headers: isUpdate ? headers : headers.map((header) => ({ name: header.name, value: header.value ?? "" })),
    };
    setSaving(true);
    const response = await fetch(
      isUpdate ? `/api/mcp-servers/${encodeURIComponent(editing.id ?? "")}` : "/api/mcp-servers",
      {
        method: isUpdate ? "PUT" : "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(body),
      },
    );
    setSaving(false);
    if (!response.ok) {
      setMessage((await response.text()) || "保存 MCP 服务失败。");
      return;
    }

    const saved = await response.json() as MCPServer;
    setServers((current) => isUpdate
      ? current.map((server) => server.id === saved.id ? saved : server)
      : [...current, saved],
    );
    setEditing(null);
    setMessage("");
  }

  async function remove() {
    if (!editing?.id || !window.confirm(`删除 MCP 服务“${editing.name}”？`)) return;

    setSaving(true);
    const response = await fetch(`/api/mcp-servers/${encodeURIComponent(editing.id)}`, { method: "DELETE" });
    setSaving(false);
    if (!response.ok) {
      setMessage((await response.text()) || "删除 MCP 服务失败。");
      return;
    }

    setServers((current) => current.filter((server) => server.id !== editing.id));
    setEditing(null);
    setMessage("");
  }

  return (
    <section className="rounded-xl border border-zinc-200 bg-white p-5">
      <div className="flex items-start justify-between gap-4">
        <div>
          <h2 className="text-base font-medium">MCP 服务</h2>
          <p className="mt-1 text-sm text-zinc-500">仅支持远程 HTTP MCP。启用后，Agent 才能在对话中调用它的工具。</p>
        </div>
        {!editing && <button className="h-9 shrink-0 rounded-lg border border-zinc-300 px-3 text-sm font-medium text-zinc-700 hover:bg-zinc-50" type="button" onClick={() => { setEditing(newDraft()); setMessage(""); }}>+ 添加服务</button>}
      </div>

      {!editing && <div className="mt-5 space-y-2">
        {servers.map((server) => (
          <button className="flex w-full items-center justify-between rounded-lg border border-zinc-200 px-3 py-3 text-left hover:bg-zinc-50" key={server.id} type="button" onClick={() => { setEditing(draftFromServer(server)); setMessage(""); }}>
            <span><span className="block text-sm font-medium">{server.name}</span><span className="mt-0.5 block text-xs text-zinc-500">{server.transport === "streamable" ? "Streamable HTTP" : "Legacy SSE"} · {server.url}</span></span>
            <span className={`text-xs ${server.enabled ? "text-emerald-700" : "text-zinc-400"}`}>{server.enabled ? "已启用" : "已停用"}</span>
          </button>
        ))}
        {servers.length === 0 && !message && <p className="rounded-lg bg-zinc-50 px-3 py-4 text-sm text-zinc-500">还没有 MCP 服务。</p>}
      </div>}

      {editing && <div className="mt-5"><MCPServerEditor draft={editing} message={message} saving={saving} onChange={setEditing} onCancel={() => { setEditing(null); setMessage(""); }} onDelete={editing.id ? remove : undefined} onSave={save} /></div>}
      {!editing && message && <p className="mt-4 text-sm text-zinc-500">{message}</p>}
    </section>
  );
}
