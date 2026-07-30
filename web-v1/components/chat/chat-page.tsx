"use client";

import { useEffect, useRef, useState } from "react";

import { AccountMenu } from "@/components/account-menu";
import { AppSidebar } from "@/components/app-sidebar";
import { SettingsDialog } from "@/components/settings/settings-dialog";
import { emptySessionUsage } from "@/lib/chat/type";
import { applyStreamEvent, errorTimelineEvent, readChatStream } from "@/lib/chat/stream";
import type {
  AgentRunStatus,
	ConversationListResponse,
  ConversationResponse,
  SessionUsage,
  Timeline,
  UserBlock,
} from "@/lib/chat/type";
import type { AvailableModelCatalogResponse, ModelInfo } from "@/lib/models/type";

import { ChatComposer } from "./chat-composer";
import { ConversationList } from "./conversation-list";
import { TimelineView } from "./timeline";

type ChatSession = {
  id: string;
  title: string;
  timeline: Timeline;
  usage: SessionUsage;
};

const emptyTimeline: Timeline = { blocks: [] };
const pendingRunsKey = "edith.pending-agent-runs";

type PendingRun = {
  requestId: string;
  sessionId: string;
};

function loadPendingRuns(): PendingRun[] {
  const value = sessionStorage.getItem(pendingRunsKey);
  if (!value) return [];

  try {
    const pendingRuns = JSON.parse(value) as unknown;
    if (!Array.isArray(pendingRuns)) return [];
    return pendingRuns.filter((run): run is PendingRun =>
      typeof run === "object" && run !== null &&
      "requestId" in run && typeof run.requestId === "string" &&
      "sessionId" in run && typeof run.sessionId === "string",
    );
  } catch {
    return [];
  }
}

function savePendingRun(pendingRun: PendingRun) {
  const pendingRuns = loadPendingRuns().filter((run) => run.requestId !== pendingRun.requestId);
  sessionStorage.setItem(pendingRunsKey, JSON.stringify([...pendingRuns, pendingRun]));
}

function removePendingRun(requestId: string) {
  const pendingRuns = loadPendingRuns().filter((run) => run.requestId !== requestId);
  sessionStorage.setItem(pendingRunsKey, JSON.stringify(pendingRuns));
}

export function ChatPage() {
  const [isRunning, setIsRunning] = useState(false);
  const [isLoadingConversations, setIsLoadingConversations] = useState(true);
  const [sessions, setSessions] = useState<ChatSession[]>([
    { id: "new-session", title: "新对话", timeline: emptyTimeline, usage: emptySessionUsage },
  ]);
  const [activeSessionID, setActiveSessionID] = useState("new-session");
  const [models, setModels] = useState<ModelInfo[]>([]);
  const [modelID, setModelID] = useState("");
  const [reasoningOptionID, setReasoningOptionID] = useState("");
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [backgroundRunSessionID, setBackgroundRunSessionID] = useState<string | null>(null);
  const liveStreamSessionIDs = useRef(new Set<string>());

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
    let stopped = false;
    const requestIDsOnPageLoad = new Set(loadPendingRuns().map((run) => run.requestId));

    async function recoverCompletedRuns() {
      for (const pendingRun of loadPendingRuns()) {
        let statusResponse: Response;
        try {
          statusResponse = await fetch(`/api/agent-runs/${encodeURIComponent(pendingRun.requestId)}`);
        } catch {
          continue;
        }
        if (statusResponse.status === 404) {
          removePendingRun(pendingRun.requestId);
          setBackgroundRunSessionID((current) => current === pendingRun.sessionId ? null : current);
          continue;
        }
        if (!statusResponse.ok) {
          continue;
        }

        const run = await statusResponse.json() as AgentRunStatus;
        if (run.status === "running") {
          if (requestIDsOnPageLoad.has(pendingRun.requestId)) {
            setBackgroundRunSessionID(pendingRun.sessionId);
          }
          continue;
        }

        let conversationResponse: Response;
        try {
          conversationResponse = await fetch(`/api/conversations/${encodeURIComponent(pendingRun.sessionId)}`);
        } catch {
          continue;
        }
        if (!conversationResponse.ok) {
          continue;
        }
        const conversation = await conversationResponse.json() as ConversationResponse;
        if (stopped) {
          return;
        }

        setSessions((current) => current.map((session) =>
          session.id === pendingRun.sessionId
            ? { ...session, timeline: conversation.timeline, usage: conversation.usage }
            : session,
        ));
        removePendingRun(pendingRun.requestId);
        setBackgroundRunSessionID((current) => current === pendingRun.sessionId ? null : current);
      }
    }

    void recoverCompletedRuns();
    const intervalID = window.setInterval(() => void recoverCompletedRuns(), 3_000);
    window.addEventListener("online", recoverCompletedRuns);
    return () => {
      stopped = true;
      window.clearInterval(intervalID);
      window.removeEventListener("online", recoverCompletedRuns);
    };
  }, []);

  useEffect(() => {
    async function loadConversations() {
      try {
        const response = await fetch("/api/conversations");
        if (!response.ok) return;
        const responseBody = await response.json() as ConversationListResponse;
        if (responseBody.conversations.length === 0) return;

        const history = responseBody.conversations.map((conversation) => ({
          ...conversation,
          timeline: emptyTimeline,
          usage: emptySessionUsage,
        }));
        setSessions(history);
        void selectSession(history[0].id);
      } finally {
        setIsLoadingConversations(false);
      }
    }
    loadConversations();
  }, []);

  function createSession() {
    const id = crypto.randomUUID();
    const session: ChatSession = {
      id,
      title: "新对话",
      timeline: { blocks: [] },
      usage: emptySessionUsage,
    };

    setSessions((current) => [session, ...current]);
    setActiveSessionID(id);
  }

  async function selectSession(sessionID: string) {
    setActiveSessionID(sessionID);
    if (liveStreamSessionIDs.current.has(sessionID)) {
      return;
    }

    const response = await fetch(`/api/conversations/${encodeURIComponent(sessionID)}`);
    if (!response.ok) return;
    const responseBody = await response.json() as ConversationResponse;
    if (liveStreamSessionIDs.current.has(sessionID)) {
      return;
    }

    setSessions((current) => current.map((session) =>
      session.id === sessionID ? { ...session, timeline: responseBody.timeline, usage: responseBody.usage } : session,
    ));
  }

  function selectModel(nextModelID: string) {
    setModelID(nextModelID);
    setReasoningOptionID("");
  }

  async function sendMessage(content: string, imageIDs: string[]) {
    if (!modelID || isRunning) {
      return;
    }

    const sessionID = activeSession.id;
    const requestID = crypto.randomUUID();
    savePendingRun({ requestId: requestID, sessionId: sessionID });
    const now = new Date().toISOString();
    const userBlock: UserBlock = {
      type: "user",
      id: crypto.randomUUID(),
      content,
      images: imageIDs.map((id) => ({ id })),
      createdAt: now,
    };
    setSessions((current) =>
      current.map((session) => {
        if (session.id !== sessionID) {
          return session;
        }

        return {
          ...session,
          title: session.timeline.blocks.length === 0 ? (content.slice(0, 18) || "图片") : session.title,
          timeline: {
            blocks: [...session.timeline.blocks, userBlock],
          },
        };
      }),
    );
    setIsRunning(true);
    liveStreamSessionIDs.current.add(sessionID);
    let streamCompleted = false;
    let taskContinues = true;
    try {
      const response = await fetch("/api/chat/stream", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          requestId: requestID,
          sessionId: sessionID,
          message: content,
          imageIds: imageIDs,
          modelId: modelID,
          ...(reasoningOptionID ? { reasoningOptionId: reasoningOptionID } : {}),
        }),
      });
      if (!response.ok) {
        if (response.status !== 409) {
          removePendingRun(requestID);
          taskContinues = false;
        }
        throw new Error(await response.text());
      }

      await readChatStream(response, (event) => {
        if (event.type === "done") {
          streamCompleted = true;
          removePendingRun(requestID);
        }
        setSessions((current) =>
          current.map((session) =>
            session.id === sessionID
              ? {
                  ...session,
                  timeline: applyStreamEvent(session.timeline, event),
                  usage: event.type === "done" && event.sessionUsage ? event.sessionUsage : session.usage,
                }
              : session,
          ),
        );
      });
      if (!streamCompleted) {
        throw new Error("网络连接已断开，任务仍在后台继续。");
      }
    } catch (error) {
      const message = error instanceof Error ? error.message : "网络连接已断开，任务仍在后台继续。";
      if (taskContinues) {
        setBackgroundRunSessionID(sessionID);
        return;
      }
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
      liveStreamSessionIDs.current.delete(sessionID);
      setIsRunning(false);
    }
  }

  return (
    <main className="flex h-screen overflow-hidden bg-zinc-50">
      <AppSidebar footer={<AccountMenu onOpenSettings={() => setSettingsOpen(true)} />}>
        <ConversationList
          activeSessionID={activeSession.id}
          isLoading={isLoadingConversations}
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
        </header>

        <TimelineView
          backgroundTaskRunning={backgroundRunSessionID === activeSession.id}
          timeline={activeSession.timeline}
        />

        <ChatComposer
          key={activeSession.id}
          isLoading={isLoadingConversations}
          isRunning={isRunning}
          sessionID={activeSession.id}
          modelID={modelID}
          models={models}
          reasoningOptionID={reasoningOptionID}
          selectedModel={selectedModel}
          sessionUsage={activeSession.usage}
          onModelChange={selectModel}
          onReasoningOptionChange={setReasoningOptionID}
          onSend={(content, imageIDs) => void sendMessage(content, imageIDs)}
        />
      </section>

      <SettingsDialog open={settingsOpen} onClose={() => setSettingsOpen(false)} />
    </main>
  );
}
