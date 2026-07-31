import Link from "next/link";

type Conversation = {
  id: string;
  title: string;
};

type ConversationListProps = {
  activeSessionID: string;
  activePage?: "chat" | "tasks";
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
    <div className="flex flex-1 flex-col p-3">
      <button
        className="mb-4 rounded-lg border border-zinc-300 px-3 py-2 text-left text-sm font-medium text-zinc-700 transition-colors hover:bg-zinc-50 disabled:cursor-wait disabled:text-zinc-400"
        disabled={isLoading}
        onClick={onCreate}
      >
        + 新对话
      </button>
      <Link
        className={`mb-4 block rounded-lg border px-3 py-2 text-left text-sm font-medium transition-colors ${
          activePage === "tasks"
            ? "border-zinc-300 bg-zinc-100 text-zinc-900"
            : "border-zinc-300 text-zinc-700 hover:bg-zinc-50"
        }`}
        href="/tasks"
      >
        定时任务
      </Link>
      <nav className="space-y-1">
        {sessions.map((session) => (
          <button
            className={`w-full truncate rounded-lg px-3 py-2 text-left text-sm transition-colors ${
              session.id === activeSessionID
                ? "bg-zinc-100 text-zinc-900"
                : "text-zinc-600 hover:bg-zinc-50"
            }`}
            key={session.id}
            onClick={() => onSelect(session.id)}
          >
            {session.title}
          </button>
        ))}
      </nav>
    </div>
  );
}
