"use client";

import { UserButton } from "@clerk/nextjs";
import { useEffect, useState } from "react";

import { AppSidebar } from "@/components/app-sidebar";
import { applyStreamEvent, errorTimelineEvent, readChatStream } from "@/lib/chat/stream";
import type {
  ConversationListResponse,
  ConversationResponse,
  Timeline,
  UserBlock,
} from "@/lib/chat/type";
import type { AvailableModelCatalogResponse, ModelInfo } from "@/lib/models/type";

import { ConversationList } from "./conversation-list";
import { TimelineView } from "./timeline";

type ChatSession = {
  id: string;
  title: string;
  timeline: Timeline;
};

const emptyTimeline: Timeline = { blocks: [] };

export function ChatPage() {
  const [message, setMessage] = useState("");
  const [isRunning, setIsRunning] = useState(false);
  const [sessions, setSessions] = useState<ChatSession[]>([
    { id: "new-session", title: "新对话", timeline: emptyTimeline },
  ]);
  const [activeSessionID, setActiveSessionID] = useState("new-session");
  const [models, setModels] = useState<ModelInfo[]>([]);
  const [modelID, setModelID] = useState("");
  const [reasoningOptionID, setReasoningOptionID] = useState("");

  const activeSession = sessions.find((session) => session.id === activeSessionID) ?? sessions[0];
  const selectedModel = models.find((model) => model.id === modelID);

  useEffect(() => {
    async function loadAvailableModels() {
      const response = await fetch("/api/available-models");
      if (!response.ok) return;
      const catalog = await response.json() as AvailableModelCatalogResponse;
      setModels(catalog.models);
      setModelID((current) => current || catalog.models[0]?.id || "");
    }
    loadAvailableModels();
  }, []);

  useEffect(() => {
    async function loadConversations() {
      const response = await fetch("/api/conversations");
      if (!response.ok) return;
      const responseBody = await response.json() as ConversationListResponse;
      if (responseBody.conversations.length === 0) return;

      const history = responseBody.conversations.map((conversation) => ({
        ...conversation,
        timeline: emptyTimeline,
      }));
      setSessions(history);
      void selectSession(history[0].id);
    }
    loadConversations();
  }, []);

  function createSession() {
    const id = crypto.randomUUID();
    const session: ChatSession = {
      id,
      title: "新对话",
      timeline: { blocks: [] },
    };

    setSessions((current) => [session, ...current]);
    setActiveSessionID(id);
    setMessage("");
  }

  async function selectSession(sessionID: string) {
    setActiveSessionID(sessionID);
    const response = await fetch(`/api/conversations/${encodeURIComponent(sessionID)}`);
    if (!response.ok) return;
    const responseBody = await response.json() as ConversationResponse;
    setSessions((current) => current.map((session) =>
      session.id === sessionID ? { ...session, timeline: responseBody.timeline } : session,
    ));
  }

  function selectModel(nextModelID: string) {
    setModelID(nextModelID);
    setReasoningOptionID("");
  }

  async function sendMessage() {
    const content = message.trim();
    if (!content || !modelID || isRunning) {
      return;
    }

    const sessionID = activeSession.id;
    const now = new Date().toISOString();
    const userBlock: UserBlock = {
      type: "user",
      id: crypto.randomUUID(),
      content,
      createdAt: now,
    };
    setSessions((current) =>
      current.map((session) => {
        if (session.id !== sessionID) {
          return session;
        }

        return {
          ...session,
          title: session.timeline.blocks.length === 0 ? content.slice(0, 18) : session.title,
          timeline: {
            blocks: [...session.timeline.blocks, userBlock],
          },
        };
      }),
    );
    setMessage("");

    setIsRunning(true);
    try {
      const response = await fetch("/api/chat/stream", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          sessionId: sessionID,
          message: content,
          modelId: modelID,
          ...(reasoningOptionID ? { reasoningOptionId: reasoningOptionID } : {}),
        }),
      });
      if (!response.ok) {
        throw new Error(await response.text());
      }

      await readChatStream(response, (event) => {
        setSessions((current) =>
          current.map((session) =>
            session.id === sessionID
              ? { ...session, timeline: applyStreamEvent(session.timeline, event) }
              : session,
          ),
        );
      });
    } catch (error) {
      const message = error instanceof Error ? error.message : "EDITH 运行失败";
      setSessions((current) =>
        current.map((session) =>
          session.id === sessionID
            ? {
                ...session,
                timeline: {
                  blocks: [...session.timeline.blocks, errorTimelineEvent(message)],
                },
              }
            : session,
        ),
      );
    } finally {
      setIsRunning(false);
    }
  }

  return (
    <main className="flex min-h-screen bg-zinc-50">
      <AppSidebar activePage="chat">
        <ConversationList
          activeSessionID={activeSession.id}
          sessions={sessions}
          onCreate={createSession}
          onSelect={selectSession}
        />
      </AppSidebar>

      <section className="flex min-w-0 flex-1 flex-col">
        <header className="flex h-16 shrink-0 items-center justify-between border-b border-zinc-200 bg-white px-5">
          <div>
            <p className="text-sm font-medium">{activeSession.title}</p>
            <p className="mt-0.5 text-xs text-zinc-500">EDITH</p>
          </div>
          <UserButton />
        </header>

        <TimelineView timeline={activeSession.timeline} />

        <form
          className="border-t border-zinc-200 bg-white p-4"
          onSubmit={(event) => {
            event.preventDefault();
            sendMessage();
          }}
        >
          <div className="mx-auto max-w-3xl rounded-2xl border border-zinc-300 bg-white p-3 shadow-sm transition-colors focus-within:border-zinc-400">
            <textarea
              className="block min-h-24 w-full resize-none bg-transparent px-1 py-1 text-sm leading-6 outline-none placeholder:text-zinc-400"
              placeholder="输入消息…"
              rows={3}
              value={message}
              onChange={(event) => setMessage(event.target.value)}
            />
            <div className="mt-2 flex items-center gap-2 border-t border-zinc-100 pt-2">
              <button className="flex h-8 w-8 items-center justify-center rounded-lg text-lg text-zinc-500 transition-colors hover:bg-zinc-100 hover:text-zinc-900" type="button" title="更多输入能力（即将支持）">+</button>
              <select className="h-8 max-w-52 rounded-lg bg-transparent px-2 text-sm font-medium text-zinc-700 outline-none hover:bg-zinc-100" disabled={isRunning || models.length === 0} value={modelID} onChange={(event) => selectModel(event.target.value)}>
                {models.length === 0 && <option value="">先在设置配置 API Key</option>}
                {models.map((model) => <option key={model.id} value={model.id}>{model.name}</option>)}
              </select>
              {selectedModel && selectedModel.reasoningOptions.length > 0 && <>
                <span className="text-zinc-300">·</span>
                <select className="h-8 rounded-lg bg-transparent px-2 text-sm text-zinc-600 outline-none hover:bg-zinc-100" disabled={isRunning} value={reasoningOptionID} onChange={(event) => setReasoningOptionID(event.target.value)}>
                  <option value="">思考</option>
                  {selectedModel.reasoningOptions.map((option) => <option key={option.id} value={option.id}>{option.name}</option>)}
                </select>
              </>}
              <button
                aria-label="发送"
                className="ml-auto flex h-9 w-9 items-center justify-center rounded-xl bg-zinc-900 text-lg text-white transition-colors hover:bg-zinc-700 disabled:cursor-not-allowed disabled:bg-zinc-200 disabled:text-zinc-400"
                disabled={!message.trim() || !modelID || isRunning}
                type="submit"
              >
                {isRunning ? "…" : "↑"}
              </button>
            </div>
          </div>
        </form>
      </section>
    </main>
  );
}
