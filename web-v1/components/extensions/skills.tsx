"use client";

import { BookOpenText, Sparkles, UserRound } from "lucide-react";
import { useEffect, useState } from "react";

import type { SkillListItem, SkillsResponse } from "@/lib/skills/type";

// Skills 负责展示当前用户实际可用的公共和用户 Skills。
export function Skills() {
  const [skills, setSkills] = useState<SkillsResponse | null>(null);
  const [message, setMessage] = useState("");

  useEffect(() => {
    async function load() {
      try {
        const response = await fetch("/api/skills", { cache: "no-store" });
        if (!response.ok) {
          setMessage("无法加载 Skills。");
          return;
        }
        setSkills(await response.json() as SkillsResponse);
      } catch {
        setMessage("无法连接 EDITH Runtime。");
      }
    }
    void load();
  }, []);

  return (
    <section className="ui-surface overflow-hidden">
      <div className="flex items-start gap-3 border-b border-border px-5 py-4">
        <span className="inline-flex size-9 shrink-0 items-center justify-center rounded-xl bg-accent-soft text-accent"><Sparkles className="size-4" /></span>
        <div>
          <h2 className="text-sm font-semibold text-ink">Skills</h2>
          <p className="mt-1 text-xs leading-5 text-muted">Agent 当前可使用的工作方法与领域知识。</p>
        </div>
      </div>

      {message && <p className="p-5 text-sm text-muted">{message}</p>}
      {!message && !skills && <p className="p-5 text-sm text-muted">加载中…</p>}
      {skills && <div className="space-y-5 p-5">
        <SkillGroup title="公共 Skills" items={skills.system} emptyMessage="暂无公共 Skills。" />
        <SkillGroup title="用户 Skills" items={skills.custom} emptyMessage="还没有用户 Skills。" />
      </div>}
    </section>
  );
}

function SkillGroup({ title, items, emptyMessage }: { title: string; items: SkillListItem[]; emptyMessage: string }) {
  const Icon = title === "公共 Skills" ? BookOpenText : UserRound;
  return (
    <div>
      <h3 className="flex items-center gap-2 text-xs font-semibold tracking-wide text-ink"><Icon className="size-3.5 text-faint" />{title}</h3>
      {items.length === 0
        ? <p className="mt-2 rounded-lg bg-surface-subtle px-3 py-4 text-sm text-muted">{emptyMessage}</p>
        : <div className="mt-2 grid gap-2 sm:grid-cols-2">
          {items.map((skill) => (
            <article className="rounded-xl border border-border bg-surface px-4 py-3.5 transition-colors hover:border-border-strong hover:bg-surface-subtle" key={skill.name}>
              <h4 className="font-mono text-xs font-semibold text-ink">{skill.name}</h4>
              <p className="mt-2 text-sm leading-6 text-muted">{skill.description}</p>
            </article>
          ))}
        </div>}
    </div>
  );
}
