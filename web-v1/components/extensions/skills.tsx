"use client";

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
    <section className="rounded-xl border border-zinc-200 bg-white p-5">
      <div>
        <h2 className="text-base font-medium">Skills</h2>
        <p className="mt-1 text-sm text-zinc-500">Agent 当前可使用的工作方法和领域知识。</p>
      </div>

      {message && <p className="mt-4 text-sm text-zinc-500">{message}</p>}
      {!message && !skills && <p className="mt-5 text-sm text-zinc-500">加载中…</p>}
      {skills && <div className="mt-5 space-y-5">
        <SkillGroup title="公共 Skills" items={skills.system} emptyMessage="暂无公共 Skills。" />
        <SkillGroup title="用户 Skills" items={skills.custom} emptyMessage="还没有用户 Skills。" />
      </div>}
    </section>
  );
}

function SkillGroup({ title, items, emptyMessage }: { title: string; items: SkillListItem[]; emptyMessage: string }) {
  return (
    <div>
      <h3 className="text-sm font-medium text-zinc-800">{title}</h3>
      {items.length === 0
        ? <p className="mt-2 rounded-lg bg-zinc-50 px-3 py-4 text-sm text-zinc-500">{emptyMessage}</p>
        : <div className="mt-2 grid gap-2 sm:grid-cols-2">
          {items.map((skill) => (
            <article className="rounded-lg border border-zinc-200 px-3 py-3" key={skill.name}>
              <h4 className="text-sm font-medium text-zinc-900">{skill.name}</h4>
              <p className="mt-1 text-sm leading-6 text-zinc-600">{skill.description}</p>
            </article>
          ))}
        </div>}
    </div>
  );
}
