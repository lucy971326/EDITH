"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { UserButton } from "@clerk/nextjs";
import { fetchEventSource } from "@microsoft/fetch-event-source";
import type { AgentEvent, ChatMessage, ModelInfo, SessionHistory, SessionInfo, StreamRequest } from "@/types/api";
import { Conversation } from "./conversation";
import { applyAgentEvent, finishStreamingMessages } from "./chat-events";

type TelegramStatus = { connected: boolean; username?: string };
type ImageDraft = { data: string; format: string; preview: string };

export default function ChatPage() {
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [sessions, setSessions] = useState<SessionInfo[]>([]);
  const [sessionID, setSessionID] = useState("");
  const [models, setModels] = useState<ModelInfo[]>([]);
  const [model, setModel] = useState("");
  const [input, setInput] = useState("");
  const [images, setImages] = useState<ImageDraft[]>([]);
  const [streaming, setStreaming] = useState(false);
  const [telegramOpen, setTelegramOpen] = useState(false);
  const [telegramToken, setTelegramToken] = useState("");
  const [telegramStatus, setTelegramStatus] = useState<TelegramStatus>({ connected: false });
  const [telegramError, setTelegramError] = useState("");
  const [telegramLoading, setTelegramLoading] = useState(false);

  const abortRef = useRef<AbortController | null>(null);
  const fileRef = useRef<HTMLInputElement>(null);
  const bottomRef = useRef<HTMLDivElement>(null);
  const currentModel = models.find((item) => item.id === model);

  const loadSessions = useCallback(async () => {
    try {
      const response = await fetch("/api/sessions");
      if (!response.ok) throw new Error("无法加载会话");
      setSessions(await response.json() as SessionInfo[]);
    } catch {
      setSessions([]);
    }
  }, []);

  const loadTelegramStatus = useCallback(async () => {
    try {
      const response = await fetch("/api/telegram/configure");
      if (!response.ok) throw new Error();
      setTelegramStatus(await response.json() as TelegramStatus);
    } catch {
      setTelegramStatus({ connected: false });
    }
  }, []);

  useEffect(() => {
    setSessionID(crypto.randomUUID());
    void loadSessions();
    void loadTelegramStatus();
    fetch("/api/models")
      .then((response) => response.json())
      .then((items: ModelInfo[]) => {
        setModels(items);
        if (items[0]) setModel(items[0].id);
      })
      .catch(() => setModels([]));
  }, [loadSessions, loadTelegramStatus]);

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [messages]);

  useEffect(() => () => abortRef.current?.abort(), []);

  function createSession() {
    if (streaming) return;
    setSessionID(crypto.randomUUID());
    setMessages([]);
  }

  async function openSession(id: string) {
    if (streaming) return;
    setSessionID(id);
    setMessages([]);
    try {
      const response = await fetch(`/api/sessions/${id}`);
      if (!response.ok) throw new Error("无法加载会话");
      const history = await response.json() as SessionHistory;
      setMessages(history.messages);
    } catch {
      setMessages([{ id: crypto.randomUUID(), kind: "error", text: "无法加载会话历史" }]);
    }
  }

  async function connectTelegram() {
    if (!telegramToken.trim()) return;
    setTelegramLoading(true);
    setTelegramError("");
    try {
      const response = await fetch("/api/telegram/configure", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ bot_token: telegramToken.trim() }),
      });
      const data = await response.json() as TelegramStatus & { error?: string };
      if (!response.ok) throw new Error(data.error || "连接失败");
      setTelegramStatus(data);
      setTelegramToken("");
    } catch (error) {
      setTelegramError(error instanceof Error ? error.message : "连接失败");
    } finally {
      setTelegramLoading(false);
    }
  }

  async function disconnectTelegram() {
    setTelegramLoading(true);
    setTelegramError("");
    try {
      const response = await fetch("/api/telegram/configure", { method: "DELETE" });
      if (!response.ok) throw new Error("断开失败");
      setTelegramStatus({ connected: false });
    } catch (error) {
      setTelegramError(error instanceof Error ? error.message : "断开失败");
    } finally {
      setTelegramLoading(false);
    }
  }

  function addImages(files: FileList | File[]) {
    Array.from(files).forEach((file) => {
      if (!file.type.startsWith("image/")) return;
      const reader = new FileReader();
      reader.onload = () => {
        const preview = String(reader.result);
        const data = preview.split(",")[1];
        if (!data) return;
        setImages((current) => [
          ...current,
          { data, format: file.type.split("/")[1], preview },
        ]);
      };
      reader.readAsDataURL(file);
    });
  }

  function stop() {
    abortRef.current?.abort();
    setStreaming(false);
    setMessages(finishStreamingMessages);
  }

  async function send() {
    const text = input.trim();
    if (streaming || !sessionID || (!text && images.length === 0)) return;

    const sentImages = images;
    setInput("");
    setImages([]);
    setStreaming(true);
    setMessages((current) => [
      ...current,
      { id: crypto.randomUUID(), kind: "user", text },
    ]);

    const request: StreamRequest = { session_id: sessionID, message: text, model };
    if (sentImages.length > 0) {
      request.images = sentImages.map(({ data, format }) => ({ data, format }));
    }

    const controller = new AbortController();
    abortRef.current = controller;

    try {
      await fetchEventSource("/api/stream", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(request),
        signal: controller.signal,
        onmessage(event) {
          const agentEvent = JSON.parse(event.data) as AgentEvent;
          setMessages((current) => applyAgentEvent(current, agentEvent));
          if (agentEvent.type === "done" || agentEvent.type === "error") controller.abort();
        },
        onerror(error) {
          throw error;
        },
      });
    } catch (error) {
      if (!controller.signal.aborted) {
        setMessages((current) => [
          ...finishStreamingMessages(current),
          { id: crypto.randomUUID(), kind: "error", text: String(error) },
        ]);
      }
    } finally {
      if (abortRef.current === controller) {
        abortRef.current = null;
        setStreaming(false);
        setMessages(finishStreamingMessages);
        void loadSessions();
      }
    }
  }

  return (
    <main className="flex h-screen min-w-0 bg-white text-slate-800">
      <SessionSidebar
        sessions={sessions}
        activeID={sessionID}
        disabled={streaming}
        onCreate={createSession}
        onOpen={openSession}
      />

      <section className="flex min-w-0 flex-1 flex-col">
        <ChatHeader
          streaming={streaming}
          sessionID={sessionID}
          models={models}
          model={model}
          onModelChange={setModel}
          onTelegram={() => setTelegramOpen((open) => !open)}
        />

        {telegramOpen && (
          <TelegramPanel
            token={telegramToken}
            status={telegramStatus}
            loading={telegramLoading}
            error={telegramError}
            onTokenChange={setTelegramToken}
            onConnect={connectTelegram}
            onDisconnect={disconnectTelegram}
          />
        )}

        <div className="min-h-0 flex-1 overflow-y-auto px-6 py-6">
          <Conversation messages={messages} />
          <div ref={bottomRef} />
        </div>

        <Composer
          value={input}
          images={images}
          showImageButton={Boolean(currentModel?.vision)}
          streaming={streaming}
          fileRef={fileRef}
          onChange={setInput}
          onAddImages={addImages}
          onRemoveImage={(index) => setImages((current) => current.filter((_, itemIndex) => itemIndex !== index))}
          onSend={send}
          onStop={stop}
        />
      </section>
    </main>
  );
}

function SessionSidebar({ sessions, activeID, disabled, onCreate, onOpen }: {
  sessions: SessionInfo[]; activeID: string; disabled: boolean; onCreate: () => void; onOpen: (id: string) => void;
}) {
  return (
    <aside className="flex w-60 shrink-0 flex-col border-r bg-slate-50">
      <div className="flex items-center justify-between border-b px-4 py-3">
        <span className="text-sm font-semibold">会话</span>
        <button type="button" onClick={onCreate} disabled={disabled} className="rounded px-2 text-xl text-slate-400 hover:bg-slate-200 hover:text-blue-600 disabled:opacity-40" title="新建会话">+</button>
      </div>
      <div className="flex-1 overflow-y-auto p-2">
        {sessions.length === 0 && <p className="p-4 text-center text-xs text-slate-400">暂无历史会话</p>}
        {sessions.map((session) => (
          <button key={session.id} type="button" disabled={disabled} onClick={() => onOpen(session.id)} className={`mb-1 w-full rounded-lg px-3 py-2 text-left text-xs transition ${session.id === activeID ? "bg-blue-100 text-blue-700" : "text-slate-500 hover:bg-slate-200"}`}>
            {relativeTime(session.updated_at)}
          </button>
        ))}
      </div>
    </aside>
  );
}

function ChatHeader({ streaming, sessionID, models, model, onModelChange, onTelegram }: {
  streaming: boolean; sessionID: string; models: ModelInfo[]; model: string; onModelChange: (model: string) => void; onTelegram: () => void;
}) {
  return (
    <header className="flex h-16 shrink-0 items-center gap-3 border-b px-6">
      <div className="flex h-9 w-9 items-center justify-center rounded-full bg-blue-600 text-sm font-bold text-white">A</div>
      <div><h1 className="font-semibold">小天</h1><p className="text-xs text-slate-400">{streaming ? "正在工作…" : "在线"}</p></div>
      <div className="ml-auto flex items-center gap-2">
        <button type="button" onClick={onTelegram} className="rounded-lg border px-2.5 py-1.5 text-xs text-slate-600 hover:border-blue-300">Telegram</button>
        {models.length > 1 && <select value={model} onChange={(event) => onModelChange(event.target.value)} className="rounded-lg border bg-white px-2 py-1.5 text-xs text-slate-600"><>{models.map((item) => <option key={item.id} value={item.id}>{item.label}</option>)}</></select>}
        <span className="hidden text-xs text-slate-300 sm:block">{sessionID.slice(0, 8)}</span>
        <UserButton />
      </div>
    </header>
  );
}

function TelegramPanel({ token, status, loading, error, onTokenChange, onConnect, onDisconnect }: {
  token: string; status: TelegramStatus; loading: boolean; error: string; onTokenChange: (value: string) => void; onConnect: () => void; onDisconnect: () => void;
}) {
  return <div className="border-b bg-blue-50 px-6 py-3"><div className="mx-auto flex max-w-4xl items-center gap-2 text-xs"><span className="shrink-0 text-slate-600">{status.connected ? `已连接 @${status.username}` : "连接你的 Telegram Bot"}</span>{!status.connected && <input type="password" value={token} onChange={(event) => onTokenChange(event.target.value)} placeholder="粘贴 Bot Token" className="min-w-0 flex-1 rounded-lg border bg-white px-2.5 py-1.5 outline-none focus:border-blue-400" />}{status.connected ? <button type="button" onClick={onDisconnect} disabled={loading} className="rounded-lg bg-red-500 px-2.5 py-1.5 text-white disabled:opacity-50">断开</button> : <button type="button" onClick={onConnect} disabled={!token.trim() || loading} className="rounded-lg bg-blue-600 px-2.5 py-1.5 text-white disabled:opacity-50">{loading ? "连接中…" : "连接"}</button>}</div>{error && <p className="mx-auto mt-1 max-w-4xl text-xs text-red-600">{error}</p>}</div>;
}

function Composer({ value, images, showImageButton, streaming, fileRef, onChange, onAddImages, onRemoveImage, onSend, onStop }: {
  value: string; images: ImageDraft[]; showImageButton: boolean; streaming: boolean; fileRef: React.RefObject<HTMLInputElement | null>; onChange: (value: string) => void; onAddImages: (files: FileList | File[]) => void; onRemoveImage: (index: number) => void; onSend: () => void; onStop: () => void;
}) {
  return (
    <footer className="border-t bg-white px-6 py-4" onDragOver={(event) => event.preventDefault()} onDrop={(event) => { event.preventDefault(); onAddImages(event.dataTransfer.files); }}>
      <div className="mx-auto max-w-4xl">
        {images.length > 0 && <div className="mb-3 flex gap-2">{images.map((image, index) => <div key={image.preview} className="relative"><img src={image.preview} alt="待发送图片" className="h-16 w-16 rounded-lg border object-cover" /><button type="button" onClick={() => onRemoveImage(index)} className="absolute -right-2 -top-2 h-5 w-5 rounded-full bg-slate-700 text-xs text-white">×</button></div>)}</div>}
        <div className="flex items-end gap-2 rounded-2xl border bg-slate-50 p-2 focus-within:border-blue-400 focus-within:bg-white">
          {showImageButton && <><input ref={fileRef} type="file" accept="image/*" multiple className="hidden" onChange={(event) => { if (event.target.files) onAddImages(event.target.files); event.target.value = ""; }} /><button type="button" disabled={streaming} onClick={() => fileRef.current?.click()} className="rounded-lg px-2 py-2 text-slate-500 hover:bg-slate-100 disabled:opacity-40" title="上传图片">🖼</button></>}
          <textarea value={value} disabled={streaming} rows={1} onChange={(event) => onChange(event.target.value)} onKeyDown={(event) => { if (event.key === "Enter" && !event.shiftKey) { event.preventDefault(); onSend(); } }} onPaste={(event) => { if (event.clipboardData.files.length) { event.preventDefault(); onAddImages(event.clipboardData.files); } }} placeholder="输入消息，Enter 发送，Shift + Enter 换行" className="max-h-40 min-h-10 flex-1 resize-none bg-transparent px-2 py-2 text-sm outline-none disabled:opacity-50" />
          {streaming ? <button type="button" onClick={onStop} className="rounded-xl bg-slate-700 px-4 py-2 text-sm text-white">停止</button> : <button type="button" onClick={onSend} disabled={!value.trim() && images.length === 0} className="rounded-xl bg-blue-600 px-4 py-2 text-sm text-white disabled:opacity-40">发送</button>}
        </div>
      </div>
    </footer>
  );
}

function relativeTime(iso: string): string {
  const minutes = Math.floor((Date.now() - new Date(iso).getTime()) / 60_000);
  if (minutes < 1) return "刚刚";
  if (minutes < 60) return `${minutes} 分钟前`;
  if (minutes < 24 * 60) return `${Math.floor(minutes / 60)} 小时前`;
  return new Date(iso).toLocaleDateString("zh-CN");
}
