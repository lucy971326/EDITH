import Link from "next/link";
import { Blocks, Clock3, MessageSquare, MessageSquarePlus } from "lucide-react";

type Conversation = {
  id: string;
  title: string;
};

type ConversationListProps = {
  activeSessionID: string;
  activePage?: "chat" | "tasks" | "extensions";
  isLoading: boolean;
  sessions: Conversation[];
  onCreate: () => void;
  onSelect: (sessionID: string) => void;
};

export function ConversationList({
  activeSessionID,
  activePage = "chat",
  isLoading,
  sessions,
  onCreate,
  onSelect,
}: ConversationListProps) {
  return (
    <div className="flex min-h-0 flex-1 flex-col p-3">
      <div className="shrink-0">
        <button
          className="mb-1 flex h-8 items-center gap-2 rounded-lg px-2.5 text-left text-sm font-medium text-muted transition-colors hover:bg-surface-subtle hover:text-ink disabled:cursor-wait disabled:opacity-60"
          disabled={isLoading}
          onClick={onCreate}
        >
          <MessageSquarePlus className="size-4" />新对话
        </button>
        <Link
          className={`mb-1 flex h-8 items-center gap-2 rounded-lg px-2.5 text-sm font-medium transition-colors ${
            activePage === "tasks"
              ? "bg-accent-soft text-accent"
              : "text-muted hover:bg-surface-subtle hover:text-ink"
          }`}
          href="/tasks"
        >
          <Clock3 className="size-4" />定时任务
        </Link>
        <Link
          className={`mb-4 flex h-8 items-center gap-2 rounded-lg px-2.5 text-sm font-medium transition-colors ${
            activePage === "extensions"
              ? "bg-accent-soft text-accent"
              : "text-muted hover:bg-surface-subtle hover:text-ink"
          }`}
          href="/extensions"
        >
          <Blocks className="size-4" />扩展
        </Link>
      </div>
      <nav aria-label="历史会话" className="min-h-0 flex-1 overflow-y-auto border-t border-border pt-3">
        <p className="mb-2 flex items-center gap-2 px-2 text-sm font-medium tracking-[0.04em] text-faint"><MessageSquare className="size-3.5" />最近会话</p>
        <div className="space-y-1">
        {sessions.map((session) => (
          <button
            className={`w-full truncate rounded-lg px-2.5 py-1.5 text-left text-sm leading-5 transition-colors ${
              session.id === activeSessionID
                ? "bg-accent-soft font-medium text-accent"
                : "text-muted hover:bg-surface-subtle hover:text-ink"
            }`}
            key={session.id}
            onClick={() => onSelect(session.id)}
          >
            {session.title}
          </button>
        ))}
        </div>
      </nav>
    </div>
  );
}
