import { Icon } from "../../ui/icon";
import type { McpServerStatus } from "../../api/mcp";
import type { SessionSummary } from "./types";

type Props = {
  sessions: SessionSummary[];
  sessionID: string;
  isRunning: boolean;
  mcps: McpServerStatus[];
  onNew: () => void;
  onSelect: (sessionID: string) => void;
  onDelete: (sessionID: string) => void;
};

function displayTime(value: string) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleString("zh-CN", { month: "numeric", day: "numeric", hour: "2-digit", minute: "2-digit" });
}

export function SessionSidebar({ sessions, sessionID, isRunning, mcps, onNew, onSelect, onDelete }: Props) {
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
        <div className="mcp-section">
          <div className="section-label"><span>扩展 · MCP</span><span>{mcps.length}</span></div>
          {mcps.length === 0 ? (
            <p className="empty-mcp">暂无 MCP server</p>
          ) : (
            mcps.map((server) => (
              <div className={`mcp-item ${server.status}`} key={server.name} title={server.error}>
                <i className="status-dot" />
                <span className="mcp-name">{server.name}</span>
                <span className="mcp-tools">{server.toolCount} 工具</span>
              </div>
            ))
          )}
        </div>
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
