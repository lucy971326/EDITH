import { useState } from "react";
import { useAguiChat } from "./hooks/useAguiChat";
import Sidebar from "./components/Sidebar";
import ChatWindow from "./components/ChatWindow";
import "./index.css";

const AGUI_ENDPOINT = "/chat";

function newThreadId() {
  return `web_${Date.now().toString(36)}`;
}

export default function App() {
  const [threadId, setThreadId] = useState(() => newThreadId());
  const [activeSessionId, setActiveSessionId] = useState<string | null>(null);

  const chat = useAguiChat(AGUI_ENDPOINT, threadId);

  const handleSelectSession = (sessionId: string) => {
    setActiveSessionId(sessionId);
    setThreadId(sessionId);
    chat.loadHistory(sessionId);
  };

  const handleNewChat = () => {
    chat.reset();
    setThreadId(newThreadId());
    setActiveSessionId(null);
  };

  return (
    <div className="app">
      <Sidebar
        activeSessionId={activeSessionId}
        onSelect={handleSelectSession}
        onNewChat={handleNewChat}
      />
      <ChatWindow
        messages={chat.messages}
        inProgress={chat.inProgress}
        lastError={chat.lastError}
        onSend={chat.send}
        onStop={chat.stop}
      />
    </div>
  );
}
