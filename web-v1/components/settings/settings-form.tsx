"use client";

import { useEffect, useState } from "react";

import type { ModelCatalogResponse, ModelInfo, ProviderInfo } from "@/lib/models/type";
import type { UserSettingsResponse } from "@/lib/userconfig/type";

import { MCPServers } from "./mcp-servers";

export function SettingsForm() {
  const [providers, setProviders] = useState<ProviderInfo[]>([]);
  const [models, setModels] = useState<ModelInfo[]>([]);
  const [keys, setKeys] = useState<Record<string, string>>({});
  const [configured, setConfigured] = useState<Record<string, boolean>>({});
  const [personality, setPersonality] = useState("");
  const [defaultModelId, setDefaultModelId] = useState("");
  const [message, setMessage] = useState("");

  useEffect(() => {
    async function load() {
      const [modelsResponse, settingsResponse] = await Promise.all([
        fetch("/api/models"),
        fetch("/api/settings"),
      ]);
      if (!modelsResponse.ok || !settingsResponse.ok) {
        setMessage("无法加载设置。");
        return;
      }

      const catalog = await modelsResponse.json() as ModelCatalogResponse;
      const settings = await settingsResponse.json() as UserSettingsResponse;
      setProviders(catalog.providers);
      setModels(catalog.models);
      setPersonality(settings.personality);
      setDefaultModelId(settings.defaultModelId);
      setConfigured(Object.fromEntries(
        settings.providers.map((provider) => [provider.providerId, provider.hasApiKey]),
      ));
    }
    void load();
  }, []);

  async function save(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();

    const providerKeys = providers.flatMap((provider) => {
      const apiKey = keys[provider.id]?.trim();
      return apiKey ? [{ providerId: provider.id, apiKey }] : [];
    });
    const response = await fetch("/api/settings", {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ personality, defaultModelId, providers: providerKeys }),
    });
    if (!response.ok) {
      setMessage((await response.text()) || "保存设置失败。");
      return;
    }

    const saved = await response.json() as UserSettingsResponse;
    setConfigured(Object.fromEntries(
      saved.providers.map((provider) => [provider.providerId, provider.hasApiKey]),
    ));
    setDefaultModelId(saved.defaultModelId);
    setKeys({});
    setMessage("已保存。");
  }

  return (
    <div className="space-y-6 p-5">
      <form className="space-y-6" onSubmit={save}>
      <section className="rounded-xl border border-zinc-200 bg-white p-5">
        <h2 className="text-base font-medium">模型供应商</h2>
        <p className="mt-1 text-sm text-zinc-500">凭据仅保存在服务端。聊天页只会显示已配置凭据的模型。</p>
        {providers.map((provider) => (
          <label className="mt-5 block text-sm font-medium" key={provider.id}>
            {provider.name}
            <input
              className="mt-2 h-10 w-full rounded-lg border border-zinc-300 px-3 text-sm outline-none focus:border-zinc-500"
              placeholder={configured[provider.id] ? "已配置；留空则不修改" : "输入 API Key"}
              type="password"
              value={keys[provider.id] ?? ""}
              onChange={(event) => setKeys((current) => ({ ...current, [provider.id]: event.target.value }))}
            />
          </label>
        ))}
      </section>

      <section className="rounded-xl border border-zinc-200 bg-white p-5">
        <h2 className="text-base font-medium">Agent</h2>
        <label className="mt-5 block text-sm font-medium">
          默认模型
          <select
            className="mt-2 h-10 w-full rounded-lg border border-zinc-300 px-3 text-sm outline-none focus:border-zinc-500"
            value={defaultModelId}
            onChange={(event) => setDefaultModelId(event.target.value)}
          >
            {models.map((model) => (
              <option key={model.id} value={model.id}>{model.name}</option>
            ))}
          </select>
          <span className="mt-1 block text-xs text-zinc-500">IM 和定时任务使用此模型；聊天页可临时选择其他模型。</span>
        </label>
        <label className="mt-5 block text-sm font-medium">
          personality
          <textarea
            className="mt-2 min-h-28 w-full rounded-lg border border-zinc-300 p-3 text-sm outline-none focus:border-zinc-500"
            value={personality}
            onChange={(event) => setPersonality(event.target.value)}
          />
        </label>
      </section>

        <div className="flex items-center justify-between">
        <p className="text-sm text-zinc-500">{message}</p>
        <button className="h-10 rounded-lg bg-zinc-900 px-4 text-sm font-medium text-white hover:bg-zinc-700" type="submit">保存</button>
        </div>
      </form>

      <MCPServers />
    </div>
  );
}
