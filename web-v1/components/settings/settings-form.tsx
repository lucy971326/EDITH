"use client";

import { CheckCircle2, KeyRound, Save, SlidersHorizontal } from "lucide-react";
import { useEffect, useState } from "react";

// browserTimezone 是用户浏览器的本地时区，首次设置时作为默认值。
function browserTimezone(): string {
  try {
    return Intl.DateTimeFormat().resolvedOptions().timeZone || "Asia/Shanghai";
  } catch {
    return "Asia/Shanghai";
  }
}

// commonTimezones 是常见 IANA 时区，保持列表精简。
const commonTimezones = [
  "Asia/Shanghai",
  "Asia/Hong_Kong",
  "Asia/Tokyo",
  "Asia/Singapore",
  "Europe/London",
  "Europe/Paris",
  "America/New_York",
  "America/Los_Angeles",
  "Australia/Sydney",
  "UTC",
];

import type { ModelCatalogResponse, ModelInfo, ProviderInfo } from "@/lib/models/type";
import type { UserSettingsResponse } from "@/lib/userconfig/type";

export function SettingsForm() {
  const [providers, setProviders] = useState<ProviderInfo[]>([]);
  const [models, setModels] = useState<ModelInfo[]>([]);
  const [keys, setKeys] = useState<Record<string, string>>({});
  const [configured, setConfigured] = useState<Record<string, boolean>>({});
  const [personality, setPersonality] = useState("");
  const [defaultModelId, setDefaultModelId] = useState("");
  const [timezone, setTimezone] = useState("");
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
      setTimezone(settings.timezone || browserTimezone());
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
      body: JSON.stringify({ personality, defaultModelId, timezone, providers: providerKeys }),
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
    <div className="p-6">
      <form className="space-y-5" onSubmit={save}>
        <section className="ui-surface overflow-hidden">
          <header className="flex items-start gap-3 border-b border-border px-5 py-4">
            <span className="inline-flex size-9 shrink-0 items-center justify-center rounded-xl bg-accent-soft text-accent"><KeyRound className="size-4" /></span>
            <div><h2 className="text-sm font-semibold text-ink">模型供应商</h2><p className="mt-1 text-xs leading-5 text-muted">凭据只存放在服务端；配置后模型才会出现在聊天页。</p></div>
          </header>
          <div className="grid gap-3 p-4 sm:grid-cols-2">
            {providers.map((provider) => (
              <label className="group rounded-xl border border-border bg-surface-subtle p-3.5 transition-colors focus-within:border-accent focus-within:bg-surface" key={provider.id}>
                <span className="mb-3 flex items-center justify-between gap-3"><span className="text-sm font-medium text-ink">{provider.name}</span>{configured[provider.id] && <span className="inline-flex items-center gap-1 text-[11px] font-medium text-success"><CheckCircle2 className="size-3.5" />已配置</span>}</span>
                <input className="ui-field h-9 px-3" placeholder={configured[provider.id] ? "留空则不修改" : "输入 API Key"} type="password" value={keys[provider.id] ?? ""} onChange={(event) => setKeys((current) => ({ ...current, [provider.id]: event.target.value }))} />
              </label>
            ))}
          </div>
        </section>

        <section className="ui-surface overflow-hidden">
          <header className="flex items-start gap-3 border-b border-border px-5 py-4">
            <span className="inline-flex size-9 shrink-0 items-center justify-center rounded-xl bg-warning-soft text-warning"><SlidersHorizontal className="size-4" /></span>
            <div><h2 className="text-sm font-semibold text-ink">Agent 偏好</h2><p className="mt-1 text-xs leading-5 text-muted">默认模型和时区会被 IM、定时任务等非聊天入口复用。</p></div>
          </header>
          <div className="grid gap-4 p-4 sm:grid-cols-2">
            <label className="block text-xs font-medium text-muted">默认模型
              <select className="ui-field mt-2 h-10 px-3" value={defaultModelId} onChange={(event) => setDefaultModelId(event.target.value)}>{models.map((model) => <option key={model.id} value={model.id}>{model.name}</option>)}</select>
            </label>
            <label className="block text-xs font-medium text-muted">时区
              <select className="ui-field mt-2 h-10 px-3" value={timezone} onChange={(event) => setTimezone(event.target.value)}>{commonTimezones.map((zone) => <option key={zone} value={zone}>{zone}</option>)}</select>
            </label>
            <label className="block text-xs font-medium text-muted sm:col-span-2">个性化指令
              <textarea className="ui-field mt-2 min-h-32 p-3 text-sm leading-6" placeholder="告诉 EDITH 你的偏好与沟通方式…" value={personality} onChange={(event) => setPersonality(event.target.value)} />
            </label>
          </div>
        </section>

        <footer className="flex items-center justify-between gap-4 rounded-xl border border-border bg-surface-subtle px-4 py-3">
          <p className="min-w-0 text-sm text-muted">{message || "修改会在下一次请求中生效。"}</p>
          <button className="ui-button-primary shrink-0" type="submit"><Save className="size-4" />保存设置</button>
        </footer>
      </form>
    </div>
  );
}
