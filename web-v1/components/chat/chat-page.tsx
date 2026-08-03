"use client";

import { useEffect, useRef, useState } from "react";
import { PanelRightClose, PanelRightOpen } from "lucide-react";

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
import { TimelineView, type TimelineRunNotice } from "./timeline";
import { SandboxPanel } from "./sandbox-panel";

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

type SessionRunNotice = {
  sessionID: string;
  reason: TimelineRunNotice;
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
  const [isCancelling, setIsCancelling] = useState(false);
  const [activeRequestID, setActiveRequestID] = useState<string | null>(null);
  const [isLoadingConversations, setIsLoadingConversations] = useState(true);
  const [sessions, setSessions] = useState<ChatSession[]>([
    { id: "new-session", title: "新对话", timeline: emptyTimeline, usage: emptySessionUsage },
  ]);
  const [activeSessionID, setActiveSessionID] = useState("new-session");
  const [models, setModels] = useState<ModelInfo[]>([]);
  const [modelID, setModelID] = useState("");
  const [reasoningOptionID, setReasoningOptionID] = useState("");
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [sandboxOpen, setSandboxOpen] = useState(false);
  const [sessionRunNotice, setSessionRunNotice] = useState<SessionRunNotice | null>(null);
  const liveStreamSessionIDs = useRef(new Set<string>());
  const abortRef = useRef<AbortController | null>(null);
  const activeRequestIDRef = useRef<string | null>(null);

  const activeSession = sessions.find((session) => session.id === activeSessionID) ?? sessions[0];
  const selectedModel = models.find((model) => model.id === modelID);

  function finishActiveRun(requestID: string) {
    if (activeRequestIDRef.current !== requestID) return;
    activeRequestIDRef.current = null;
    setActiveRequestID(null);
    setIsRunning(false);
    setIsCancelling(false);
  }

  function clearSessionRunNotice(sessionID: string) {
    setSessionRunNotice((current) => current?.sessionID === sessionID ? null : current);
  }

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
        if (!statusResponse.ok && statusResponse.status !== 404) {
          continue;
        }

        if (statusResponse.ok) {
          const run = await statusResponse.json() as AgentRunStatus;
          if (run.status !== "running") {
            continue;
          }
          if (requestIDsOnPageLoad.has(pendingRun.requestId)) {
            setSessionRunNotice({ sessionID: pendingRun.sessionId, reason: "stream_disconnected" });
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
          if (conversationResponse.status === 404) {
            // Runner 和会话都不存在时，这条 pending 记录已经失效，停止轮询。
            removePendingRun(pendingRun.requestId);
            finishActiveRun(pendingRun.requestId);
          }
          continue;
        }
        const conversation = await conversationResponse.json() as ConversationResponse;
        // 会话仍在实时流式输出时，不拿历史覆盖，避免与 SSE 事件竞争丢块。
        if (liveStreamSessionIDs.current.has(pendingRun.sessionId)) {
          continue;
        }
        if (stopped) {
          return;
        }

        setSessions((current) => current.map((session) =>
          session.id === pendingRun.sessionId
            ? { ...session, timeline: conversation.timeline, usage: conversation.usage }
            : session,
        ));
        removePendingRun(pendingRun.requestId);
        clearSessionRunNotice(pendingRun.sessionId);
        finishActiveRun(pendingRun.requestId);
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
        const requestedSessionID = new URLSearchParams(window.location.search).get("sessionId");
        const initialSessionID = history.some((session) => session.id === requestedSessionID)
          ? requestedSessionID!
          : history[0].id;
        void selectSession(initialSessionID);
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

  async function cancelRun() {
    if (!activeRequestID || isCancelling) return;
    abortRef.current?.abort();
    setIsCancelling(true);
    try {
      const response = await fetch(`/api/agent-runs/${encodeURIComponent(activeRequestID)}`, { method: "POST" });
      if (!response.ok) {
        throw new Error("取消请求未被服务端接受");
      }
    } catch {
      setIsCancelling(false);
      setSessionRunNotice({ sessionID: activeSessionID, reason: "stream_disconnected" });
      return; // 服务端是否收到取消请求未知，保留 pending 并继续查询后端状态
    }
    // 204 只表示 Runner 已收到取消信号。继续保留 pending，直到
    // ManagedRunner 不再报告该 requestID 为运行中，才能确认已结束。
  }

  async function sendMessage(content: string, imageIDs: string[]) {
    if (!modelID || isRunning) {
      return;
    }

    const sessionID = activeSession.id;
    const requestID = crypto.randomUUID();
    activeRequestIDRef.current = requestID;
    setActiveRequestID(requestID);
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
    const abort = new AbortController();
    abortRef.current = abort;
    let streamCompleted = false;
    let taskContinues = true;
    try {
      const response = await fetch("/api/chat/stream", {
        signal: abort.signal,
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
        if (response.status === 409) {
          // 该会话已有任务在运行，这条消息没有被接受。
          // 保留刚插入的用户消息块，只显示“会话忙”提示，避免用户以为自己的消息丢了。
          removePendingRun(requestID);
          setSessionRunNotice({ sessionID, reason: "session_busy" });
          taskContinues = false;
          finishActiveRun(requestID);
          return;
        }

        removePendingRun(requestID);
        taskContinues = false;
        finishActiveRun(requestID);
        throw new Error(await response.text());
      }

      await readChatStream(response, (event) => {
        if (event.type === "run.completed" || event.type === "run.canceled") {
          streamCompleted = true;
          removePendingRun(requestID);
          finishActiveRun(requestID);
        }
        setSessions((current) =>
          current.map((session) =>
            session.id === sessionID
              ? {
                  ...session,
                  timeline: applyStreamEvent(session.timeline, event),
                  usage: (event.type === "run.completed" || event.type === "run.canceled") && event.sessionUsage ? event.sessionUsage : session.usage,
                }
              : session,
          ),
        );
      });
      if (!streamCompleted) {
        throw new Error("网络连接已断开，任务仍在后台继续。");
      }
    } catch (error) {
      if (error instanceof DOMException && error.name === "AbortError") {
        return; // 用户主动取消，不发错误提示
      }
      const message = error instanceof Error ? error.message : "网络连接已断开，任务仍在后台继续。";
      if (taskContinues) {
        setSessionRunNotice({ sessionID, reason: "stream_disconnected" });
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
      if (!abort.signal.aborted) {
        setIsRunning(false);
      }
      if (!taskContinues) {
        finishActiveRun(requestID);
      }
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
          <button
            className="rounded-md p-2 text-zinc-500 hover:bg-zinc-100 hover:text-zinc-900"
            onClick={() => setSandboxOpen((current) => !current)}
            title={sandboxOpen ? "关闭 Sandbox 文件" : "展开 Sandbox 文件"}
            aria-label={sandboxOpen ? "关闭 Sandbox 文件" : "展开 Sandbox 文件"}
          >
            {sandboxOpen ? <PanelRightClose className="size-5" /> : <PanelRightOpen className="size-5" />}
          </button>
        </header>

        <TimelineView
          runNotice={sessionRunNotice?.sessionID === activeSession.id ? sessionRunNotice.reason : undefined}
          timeline={activeSession.timeline}
        />

        <ChatComposer
          key={activeSession.id}
          isLoading={isLoadingConversations}
          isRunning={isRunning}
          isCancelling={isCancelling}
          sessionID={activeSession.id}
          modelID={modelID}
          models={models}
          reasoningOptionID={reasoningOptionID}
          selectedModel={selectedModel}
          sessionUsage={activeSession.usage}
          onCancel={cancelRun}
          onModelChange={selectModel}
          onReasoningOptionChange={setReasoningOptionID}
          onSend={(content, imageIDs) => void sendMessage(content, imageIDs)}
        />
      </section>

      <SandboxPanel key={activeSession.id} sessionID={activeSession.id} open={sandboxOpen} />

      <SettingsDialog open={settingsOpen} onClose={() => setSettingsOpen(false)} />
    </main>
  );
}
