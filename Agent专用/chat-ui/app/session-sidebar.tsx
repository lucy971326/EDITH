"use client";

import { useEffect, useState } from "react";

// 后端 Go 服务的地址（与 .env.local / .env 一致）
const API_BASE = process.env.NEXT_PUBLIC_AGUI_API ?? "http://127.0.0.1:8080";

interface SessionItem {
  id: string;
  updated_at: string;
  event_count: number;
  title: string;
}

// 侧边栏：列出会话 + 新建按钮 + 点击切换
export function SessionSidebar({
  activeId,
  onSelect,
  onNewChat,
  refreshKey,
}: {
  activeId: string | null;
  onSelect: (id: string) => void;
  onNewChat: () => void;
  refreshKey: number; // 父组件加 1 → 重新拉列表
}) {
  const [sessions, setSessions] = useState<SessionItem[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let stop = false;
    const tick = () => {
      fetch(`${API_BASE}/api/sessions`)
        .then((r) => r.json())
        .then((data) => {
          if (stop) return;
          setSessions(data.sessions ?? []);
          setLoading(false);
        })
        .catch(() => !stop && setLoading(false));
    };
    tick();
    // 5s 轮询一次 (列表小, 压力可忽略; 比监听 v2 内部 hook 简单 100 倍)
    const id = setInterval(tick, 5000);
    return () => {
      stop = true;
      clearInterval(id);
    };
  }, [refreshKey]);

  return (
    <aside
      style={{
        width: 240,
        borderRight: "1px solid #333",
        padding: 12,
        display: "flex",
        flexDirection: "column",
        gap: 8,
        overflowY: "auto",
      }}
    >
      <button
        onClick={onNewChat}
        style={{
          padding: "8px 12px",
          background: "#1f6feb",
          color: "white",
          border: "none",
          borderRadius: 6,
          cursor: "pointer",
        }}
      >
        + 新对话
      </button>

      {loading && <div style={{ opacity: 0.6 }}>加载中…</div>}

      {!loading && sessions.length === 0 && (
        <div style={{ opacity: 0.6, fontSize: 12 }}>
          还没有会话 — 发条消息就有了
        </div>
      )}

      <ul style={{ listStyle: "none", padding: 0, margin: 0 }}>
        {sessions.map((s) => {
          const isActive = activeId === s.id;
          return (
            <li key={s.id}>
              <button
                onClick={() => onSelect(s.id)}
                style={{
                  width: "100%",
                  textAlign: "left",
                  padding: "6px 8px",
                  marginTop: 2,
                  background: isActive ? "#1f6feb" : "transparent",
                  color: isActive ? "white" : "inherit",
                  border: "none",
                  borderRadius: 4,
                  cursor: "pointer",
                  fontSize: 12,
                }}
                title={`${s.id} · ${s.event_count} 条消息`}
              >
                <div
                  style={{
                    overflow: "hidden",
                    textOverflow: "ellipsis",
                    whiteSpace: "nowrap",
                  }}
                >
                  {s.title.slice(0, 16)}
                </div>
                <div style={{ opacity: 0.6, fontSize: 10 }}>
                  {s.event_count} 条 · {new Date(s.updated_at).toLocaleString()}
                </div>
              </button>
            </li>
          );
        })}
      </ul>
    </aside>
  );
}