"use client";

import { FormEvent, useEffect, useState } from "react";
import { cancelRun, startRun } from "../../api/agent";
import { deleteSession, getSession, listSessions } from "../../api/sessions";
import { getModels, type ModelCatalog } from "../../api/models";
import { readSSEFrames, type AssistantBlock, type StreamEvent } from "../../lib/stream";
import { Composer } from "../composer/composer";
import { FilesPanel } from "../files/files-panel";
import { RuntimeStatus } from "../runtime/runtime-status";
import { SessionSidebar } from "../sessions/session-sidebar";
import type { SessionSummary } from "../sessions/types";
import { ChatTimeline } from "./chat-timeline";
import type { AssistantMessage, ChatMessage } from "./types";

function newID() {
  return crypto.randomUUID();
}

function applyEvent(message: AssistantMessage, streamEvent: StreamEvent): AssistantMessage {
  if (streamEvent.type === "message.delta" || streamEvent.type === "reasoning.delta") {
    if (!streamEvent.blockId || !streamEvent.blockType || !streamEvent.delta) {
      return message;
    }
    const current = message.blocks.find((block) => block.id === streamEvent.blockId);
    if (current?.type === "text" || current?.type === "reasoning") {
      return {
        ...message,
        blocks: message.blocks.map((block) =>
          block.id === streamEvent.blockId && (block.type === "text" || block.type === "reasoning")
            ? { ...block, content: block.content + streamEvent.delta }
            : block,
        ),
      };
    }
    return {
      ...message,
      blocks: [...message.blocks, { id: streamEvent.blockId, type: streamEvent.blockType, content: streamEvent.delta }],
    };
  }
  if (!streamEvent.type.startsWith("tool.") || !streamEvent.toolCallId) {
    return message;
  }
  const current = message.blocks.find(
    (block): block is Extract<AssistantBlock, { type: "tool" }> => block.id === streamEvent.toolCallId && block.type === "tool",
  );
  const next: Extract<AssistantBlock, { type: "tool" }> = {
    id: streamEvent.toolCallId,
    type: "tool",
    name: streamEvent.toolName ?? current?.name ?? "工具",
    arguments: streamEvent.arguments ?? current?.arguments ?? "",
    result: streamEvent.toolResult ?? current?.result ?? "",
    status: streamEvent.toolStatus ?? current?.status ?? "requested",
  };
  return {
    ...message,
    blocks: current ? message.blocks.map((block) => (block.id === next.id ? next : block)) : [...message.blocks, next],
  };
}

export function Workbench() {
  const [sessionID, setSessionID] = useState(newID);
  const [sessions, setSessions] = useState<SessionSummary[]>([]);
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [input, setInput] = useState("");
  const [requestID, setRequestID] = useState<string | null>(null);
  const [modelCatalog, setModelCatalog] = useState<ModelCatalog | null>(null);
  const [modelID, setModelID] = useState("");
  const [thinkingMode, setThinkingMode] = useState("");
  const [error, setError] = useState("");
  const [isStopping, setIsStopping] = useState(false);
  const isRunning = requestID !== null;
  const currentSession = sessions.find((session) => session.id === sessionID);

  async function refreshSessions() {
    try {
      setSessions(await listSessions());
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "无法读取会话列表");
    }
  }

  useEffect(() => {
    void listSessions()
      .then(setSessions)
      .catch((cause: unknown) => setError(cause instanceof Error ? cause.message : "无法读取会话列表"));
  }, []);

  useEffect(() => {
    void getModels()
      .then((catalog) => {
        setModelCatalog(catalog);
        setModelID(catalog.defaultModelId);
        const defaultModel = catalog.models.find((model) => model.id === catalog.defaultModelId);
        setThinkingMode(defaultModel?.thinking.defaultMode ?? "");
      })
      .catch((cause: unknown) => setError(cause instanceof Error ? cause.message : "无法读取模型目录"));
  }, []);

  function selectModel(nextModelID: string) {
    setModelID(nextModelID);
    const nextModel = modelCatalog?.models.find((model) => model.id === nextModelID);
    setThinkingMode(nextModel?.thinking.defaultMode ?? "");
  }

  function newSession() {
    if (isRunning) return;
    setSessionID(newID());
    setMessages([]);
    setError("");
  }

  async function selectSession(nextSessionID: string) {
    if (isRunning || nextSessionID === sessionID) return;
    try {
      const history = await getSession(nextSessionID);
      setSessionID(nextSessionID);
      setMessages(history.messages);
      setError("");
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "无法读取会话历史");
    }
  }

  async function removeSession(targetSessionID: string) {
    if (isRunning || !window.confirm("删除这条会话？此操作无法恢复。")) return;
    try {
      await deleteSession(targetSessionID);
      if (targetSessionID === sessionID) newSession();
      await refreshSessions();
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "无法删除会话");
    }
  }

  function applyStreamEvent(assistantID: string, streamEvent: StreamEvent) {
    if (streamEvent.type === "run.error") {
      setError(streamEvent.error?.message ?? "Agent 请求失败");
    }
    setMessages((current) =>
      current.map((message) =>
        message.id === assistantID && message.role === "assistant" ? applyEvent(message, streamEvent) : message,
      ),
    );
  }

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const message = input.trim();
    if (!message || isRunning || !modelCatalog || !modelID || !thinkingMode) {
      return;
    }
    const nextRequestID = newID();
    const assistantID = newID();
    setMessages((current) => [
      ...current,
      { id: newID(), role: "user", content: message },
      { id: assistantID, role: "assistant", blocks: [] },
    ]);
    setInput("");
    setError("");
    setRequestID(nextRequestID);
    try {
      const response = await startRun({
        requestId: nextRequestID,
        sessionId: sessionID,
        message,
        modelId: modelID,
        thinkingMode,
      });
      if (!response.ok || !response.body) {
        const result = await response.json().catch(() => null) as { message?: string } | null;
        throw new Error(result?.message ?? "无法启动 Agent 请求");
      }
      const reader = response.body.getReader();
      const decoder = new TextDecoder();
      let buffer = "";
      while (true) {
        const { done, value } = await reader.read();
        if (done) {
          break;
        }
        buffer += decoder.decode(value, { stream: true });
        const frames = readSSEFrames(buffer);
        buffer = frames.rest;
        frames.events.forEach((streamEvent) => applyStreamEvent(assistantID, streamEvent));
      }
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "Agent 请求失败");
    } finally {
      setRequestID(null);
      setIsStopping(false);
      void refreshSessions();
    }
  }

  async function stop() {
    if (!requestID || isStopping) {
      return;
    }
    setIsStopping(true);
    try {
      if (!(await cancelRun(requestID)).ok) {
        throw new Error("停止请求失败");
      }
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "停止请求失败");
      setIsStopping(false);
    }
  }

  return (
    <main className="workbench">
      <SessionSidebar sessions={sessions} sessionID={sessionID} isRunning={isRunning} onNew={newSession} onSelect={selectSession} onDelete={removeSession} />
      <section className="chat-area">
        <header className="chat-header">
          <span className="chat-title">{currentSession?.title ?? "当前临时会话"}</span>
          <RuntimeStatus isRunning={isRunning} requestID={requestID} />
        </header>
        <section className="messages">
          <ChatTimeline isRunning={isRunning} messages={messages} />
        </section>
        {error && <p className="error">{error}</p>}
        <Composer
          input={input}
          isRunning={isRunning}
          isStopping={isStopping}
          modelCatalog={modelCatalog}
          modelID={modelID}
          thinkingMode={thinkingMode}
          onInput={setInput}
          onModelChange={selectModel}
          onThinkingModeChange={setThinkingMode}
          onStop={stop}
          onSubmit={submit}
        />
      </section>
      <FilesPanel />
    </main>
  );
}
