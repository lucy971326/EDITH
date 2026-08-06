import { Icon } from "../../ui/icon";
import type { SessionSummary } from "./types";

type Props = {
  sessions: SessionSummary[];
  sessionID: string;
  isRunning: boolean;
  onNew: () => void;
  onSelect: (sessionID: string) => void;
  onDelete: (sessionID: string) => void;
};

function displayTime(value: string) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleString("zh-CN", { month: "numeric", day: "numeric", hour: "2-digit", minute: "2-digit" });
}

export function SessionSidebar({ sessions, sessionID, isRunning, onNew, onSelect, onDelete }: Props) {
  return (
    <aside className="sidebar">
      <div className="brand">
        <div className="brand-mark">E</div>
        <div>
          <div className="brand-name">EDITH</div>
          <div className="brand-subtitle">Local Coding Agent</div>
        </div>
      </div>
      <div className="sidebar-main">
        <button className="new-chat" type="button" disabled={isRunning} onClick={onNew}>
          <Icon name="plus" />新建会话 <span className="shortcut">Ctrl K</span>
        </button>
        <div className="section-label"><span>当前目录 · 会话</span><span>{sessions.length}</span></div>
        {sessions.map((session) => (
          <div className={`session-item ${session.id === sessionID ? "active" : ""}`} key={session.id}>
            <button type="button" disabled={isRunning} onClick={() => onSelect(session.id)}>
              <span className="session-title">{session.title}</span>
              <span className="session-meta">{displayTime(session.updatedAt)}</span>
            </button>
            <button aria-label={`删除 ${session.title}`} className="session-delete" disabled={isRunning} type="button" onClick={() => onDelete(session.id)}>
              ×
            </button>
          </div>
        ))}
        {sessions.length === 0 && <p className="empty-sessions">当前目录还没有已保存的会话。</p>}
      </div>
      <div className="sidebar-footer">
        <button className="nav-button" type="button"><Icon name="grid" />扩展 <span className="soon-label">即将支持</span></button>
        <button className="nav-button" type="button"><Icon name="settings" />设置 <span className="soon-label">即将支持</span></button>
        <div className="user-card">
          <div className="avatar">E</div>
          <div>
            <strong>本地工作区</strong>
            <div className="workspace-name">仅当前启动目录</div>
          </div>
        </div>
      </div>
    </aside>
  );
}
