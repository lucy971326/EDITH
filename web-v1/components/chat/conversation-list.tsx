type Conversation = {
  id: string;
  title: string;
};

type ConversationListProps = {
  activeSessionID: string;
  sessions: Conversation[];
  onCreate: () => void;
  onSelect: (sessionID: string) => void;
};

export function ConversationList({
  activeSessionID,
  sessions,
  onCreate,
  onSelect,
}: ConversationListProps) {
  return (
    <div className="flex flex-1 flex-col p-3">
      <button
        className="mb-4 rounded-lg border border-zinc-300 px-3 py-2 text-left text-sm font-medium text-zinc-700 transition-colors hover:bg-zinc-50"
        onClick={onCreate}
      >
        + 新对话
      </button>
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
