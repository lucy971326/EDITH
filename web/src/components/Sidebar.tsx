import { useEffect, useState, useCallback } from "react";
import { Button, Tag } from "tdesign-react";
import { FolderIcon, ChatIcon } from "tdesign-icons-react";

type SessionMeta = {
  id: string;
  updatedAt: string;
};

type RepoGroup = {
  repo: string;
  sessions: SessionMeta[];
};

function groupByRepo(sessions: SessionMeta[]): RepoGroup[] {
  const map = new Map<string, SessionMeta[]>();
  for (const s of sessions) {
    let repo: string;
    if (s.id.startsWith("web_")) {
      repo = "Web 对话";
    } else if (s.id.includes("#")) {
      repo = s.id.split("#")[0];
    } else {
      repo = s.id;
    }
    const list = map.get(repo) ?? [];
    list.push(s);
    map.set(repo, list);
  }
  return Array.from(map.entries())
    .map(([repo, sessions]) => ({ repo, sessions }))
    .sort((a, b) => {
      if (a.repo === "Web 对话") return -1;
      if (b.repo === "Web 对话") return 1;
      return a.repo.localeCompare(b.repo);
    });
}

export default function Sidebar({
  activeSessionId,
  onSelect,
  onNewChat,
}: {
  activeSessionId: string | null;
  onSelect: (id: string) => void;
  onNewChat: () => void;
}) {
  const [groups, setGroups] = useState<RepoGroup[]>([]);

  const fetchSessions = useCallback(async () => {
    try {
      const r = await fetch("/api/sessions");
      const data = await r.json();
      setGroups(groupByRepo(Array.isArray(data) ? data : []));
    } catch {}
  }, []);

  useEffect(() => {
    fetchSessions();
    const interval = setInterval(fetchSessions, 3000);
    return () => clearInterval(interval);
  }, [fetchSessions]);

  return (
    <aside className="sidebar">
      <div className="sidebar__header">
        <strong>🔧 EDITH</strong>
        <Button size="small" onClick={onNewChat}>
          ＋
        </Button>
      </div>
      <div className="sidebar__list">
        {groups.length === 0 && (
          <div className="sidebar__empty">暂无会话</div>
        )}
        {groups.map((g) => (
          <div key={g.repo} className="sidebar__group">
            <div className="sidebar__group-title">
              <FolderIcon /> {g.repo}
            </div>
            {g.sessions.map((s) => {
              const isWeb = s.id.startsWith("web_");
              const label = isWeb
                ? s.id.slice(4, 12)
                : s.id.includes("#") ? `#${s.id.split("#")[1]}` : s.id;
              return (
                <div
                  key={s.id}
                  className={`sidebar__item ${s.id === activeSessionId ? "sidebar__item--active" : ""}`}
                  onClick={() => onSelect(s.id)}
                >
                  <ChatIcon />
                  <span>{label}</span>
                </div>
              );
            })}
          </div>
        ))}
      </div>
    </aside>
  );
}
