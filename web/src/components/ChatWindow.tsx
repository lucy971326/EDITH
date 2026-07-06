import { useRef, useEffect, useState } from "react";
import { Button, Textarea } from "tdesign-react";
import { ChatMarkdown } from "@tdesign-react/chat";
import type { UiMessage } from "../hooks/useAguiChat";

function formatJson(raw?: string): string {
  if (!raw) return "";
  try {
    const parsed = JSON.parse(raw);
    return JSON.stringify(parsed, null, 2);
  } catch {
    return raw;
  }
}

function ChatBubble({ msg, onToolToggle, openToolIds }: {
  msg: UiMessage;
  onToolToggle: (id: string) => void;
  openToolIds: Set<string>;
}) {
  if (msg.kind === "thinking") {
    const isStreaming = msg.status === "streaming";
    return (
      <div className="message__bubble message__bubble--thinking">
        <div className="message__thinking-header">
          💭 {msg.title ?? "思考中..."}
          {isStreaming && <span className="cursor-blink">|</span>}
        </div>
        <div className="message__thinking-content">{msg.content}</div>
      </div>
    );
  }

  if (msg.kind === "tool-call") {
    const isOpen = openToolIds.has(msg.id);
    const args = formatJson(msg.toolCall?.args);
    const result = formatJson(msg.toolCall?.result);
    return (
      <div className="message__bubble message__bubble--tool">
        <details open={isOpen} onToggle={() => onToolToggle(msg.id)}>
          <summary className="toolcall__summary">
            <span>🔧 {msg.title ?? "tool"}</span>
            <Tag status={msg.status === "complete" ? "success" : "primary"}>
              {msg.status === "complete" ? "完成" : "执行中"}
            </Tag>
          </summary>
          <div className="toolcall__panels">
            {args && (
              <pre className="toolcall__code">{args}</pre>
            )}
            {result && (
              <>
                <div className="toolcall__label">结果:</div>
                <pre className="toolcall__code">{result}</pre>
              </>
            )}
          </div>
        </details>
      </div>
    );
  }

  if (msg.role === "user") {
    return <ChatMarkdown content={msg.content} />;
  }

  return <ChatMarkdown content={msg.content} />;
}

function Tag({ status, children }: { status?: string; children: React.ReactNode }) {
  const cls = status === "success" ? "tag tag--success" : "tag tag--primary";
  return <span className={cls}>{children}</span>;
}

export default function ChatWindow({
  messages,
  inProgress,
  lastError,
  onSend,
  onStop,
}: {
  messages: UiMessage[];
  inProgress: boolean;
  lastError: string | null;
  onSend: (text: string) => void;
  onStop: () => void;
}) {
  const [input, setInput] = useState("");
  const [openToolIds, setOpenToolIds] = useState<Set<string>>(new Set());
  const chatRef = useRef<HTMLDivElement>(null);
  const autoScrollRef = useRef(true);

  // 智能滚动：用户没手动上翻就自动到底部
  useEffect(() => {
    if (chatRef.current && autoScrollRef.current) {
      chatRef.current.scrollTop = chatRef.current.scrollHeight;
    }
  }, [messages]);

  const handleScroll = () => {
    if (!chatRef.current) return;
    const { scrollTop, scrollHeight, clientHeight } = chatRef.current;
    autoScrollRef.current = scrollHeight - scrollTop - clientHeight < 80;
  };

  const handleSend = () => {
    const text = input.trim();
    if (!text || inProgress) return;
    setInput("");
    onSend(text);
  };

  const toggleTool = (id: string) => {
    setOpenToolIds((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  };

  return (
    <div className="chat-area">
      <div className="chat-area__header">
        <strong>EDITH 对话</strong>
        {inProgress && (
          <Button size="small" variant="outline" onClick={onStop}>
            停止
          </Button>
        )}
      </div>

      <div className="chat-area__messages" ref={chatRef} onScroll={handleScroll}>
        {messages.length === 0 && (
          <div className="chat-area__empty">输入消息开始对话</div>
        )}
        {messages.map((msg) => (
          <div key={msg.id} className={`chat-message ${msg.role === "user" ? "chat-message--user" : ""}`}>
            <div className="chat-message__label">{msg.role === "user" ? "你" : "EDITH"}</div>
            <ChatBubble msg={msg} onToolToggle={toggleTool} openToolIds={openToolIds} />
          </div>
        ))}
        {lastError && <div className="chat-area__error">⚠️ {lastError}</div>}
      </div>

      <div className="chat-area__input">
        <Textarea
          value={input}
          onChange={(v: any) => setInput(String(v))}
          placeholder="输入消息..."
          autosize={{ minRows: 2, maxRows: 6 }}
          disabled={inProgress}
          onKeydown={(_, ctx) => {
            const e = ctx.e as any;
            if (e.key === "Enter" && !e.shiftKey) {
              e.preventDefault();
              handleSend();
            }
          }}
        />
        <Button theme="primary" onClick={handleSend} disabled={inProgress}>
          发送
        </Button>
      </div>
    </div>
  );
}
