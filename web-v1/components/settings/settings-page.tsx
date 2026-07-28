"use client";

import { UserButton } from "@clerk/nextjs";
import { useEffect, useState } from "react";

import { AppSidebar } from "@/components/app-sidebar";
import type { ModelCatalogResponse, ProviderInfo } from "@/lib/models/type";
import type { UserSettingsResponse } from "@/lib/userconfig/type";

export function SettingsPage() {
  const [providers, setProviders] = useState<ProviderInfo[]>([]);
  const [keys, setKeys] = useState<Record<string, string>>({});
  const [configured, setConfigured] = useState<Record<string, boolean>>({});
  const [personality, setPersonality] = useState("");
  const [message, setMessage] = useState("");

  useEffect(() => {
    async function load() {
      const [modelsResponse, settingsResponse] = await Promise.all([fetch("/api/models"), fetch("/api/settings")]);
      if (!modelsResponse.ok || !settingsResponse.ok) { setMessage("无法加载设置。"); return; }
      const catalog = await modelsResponse.json() as ModelCatalogResponse;
      const settings = await settingsResponse.json() as UserSettingsResponse;
      setProviders(catalog.providers);
      setPersonality(settings.personality);
      setConfigured(Object.fromEntries(settings.providers.map((provider) => [provider.providerId, provider.hasApiKey])));
    }
    load();
  }, []);

  async function save(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const body = { personality, providers: providers.flatMap((provider) => keys[provider.id]?.trim() ? [{ providerId: provider.id, apiKey: keys[provider.id].trim() }] : []) };
    const response = await fetch("/api/settings", { method: "PUT", headers: { "Content-Type": "application/json" }, body: JSON.stringify(body) });
    if (!response.ok) {
      const error = await response.text();
      setMessage(error || "保存设置失败。");
      return;
    }
    const saved = await response.json() as UserSettingsResponse;
    setConfigured(Object.fromEntries(saved.providers.map((provider) => [provider.providerId, provider.hasApiKey])));
    setKeys({}); setMessage("已保存。");
  }

  return <main className="flex min-h-screen bg-zinc-50"><AppSidebar activePage="settings"><div className="flex-1" /></AppSidebar><section className="flex min-w-0 flex-1 flex-col"><header className="flex h-16 items-center justify-between border-b border-zinc-200 bg-white px-5"><div><p className="text-sm font-medium">设置</p><p className="mt-0.5 text-xs text-zinc-500">供应商凭据与 Agent 配置</p></div><UserButton /></header><div className="flex-1 overflow-y-auto p-5 md:p-8"><form className="mx-auto max-w-xl space-y-6" onSubmit={save}><section className="rounded-xl border border-zinc-200 bg-white p-5"><h1 className="text-base font-medium">模型供应商</h1><p className="mt-1 text-sm text-zinc-500">凭据仅保存在服务端。聊天页只会显示已配置凭据的模型。</p>{providers.map((provider) => <label className="mt-5 block text-sm font-medium" key={provider.id}>{provider.name}<input className="mt-2 h-10 w-full rounded-lg border border-zinc-300 px-3 text-sm" placeholder={configured[provider.id] ? "已配置；留空则不修改" : "输入 API Key"} type="password" value={keys[provider.id] ?? ""} onChange={(event) => setKeys((current) => ({ ...current, [provider.id]: event.target.value }))} /></label>)}</section><section className="rounded-xl border border-zinc-200 bg-white p-5"><h2 className="text-base font-medium">Agent</h2><label className="mt-5 block text-sm font-medium">personality<textarea className="mt-2 min-h-28 w-full rounded-lg border border-zinc-300 p-3 text-sm" value={personality} onChange={(event) => setPersonality(event.target.value)} /></label></section><div className="flex items-center justify-between"><p className="text-sm text-zinc-500">{message}</p><button className="h-10 rounded-lg bg-zinc-900 px-4 text-sm font-medium text-white" type="submit">保存</button></div></form></div></section></main>;
}
