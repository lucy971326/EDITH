"use client";

import { FormEvent, useEffect, useRef, useState } from "react";
import { executeCommand, listCommands, type CommandDefinition } from "../../api/commands";
import { cancelRun, startRun } from "../../api/agent";
import { deleteSession, getSession, getSessionContext, listSessions } from "../../api/sessions";
import { getModels, type ModelCatalog } from "../../api/models";
import { getMcpStatus, type McpServerStatus } from "../../api/mcp";
import { getSkills, type SkillEntry } from "../../api/skills";
import { consumeSSE, type StreamEvent } from "../../lib/stream";
import { Icon } from "../../ui/icon";
import { Composer, type PendingImage } from "../composer/composer";
import { FilesPanel } from "../files/files-panel";
import { RuntimeStatus } from "../runtime/runtime-status";
import { SessionSidebar } from "../sessions/session-sidebar";
import type { SessionSummary } from "../sessions/types";
import { applyEvent } from "./apply-event";
import { ChatTimeline } from "./chat-timeline";
import type { ChatMessage } from "./types";

const SIDEBAR_MIN = 160;
const SIDEBAR_MAX = 420;
const INSPECTOR_MIN = 360;
const INSPECTOR_MAX = 900;

function clamp(value: number, min: number, max: number) {
  return Math.min(max, Math.max(min, value));
}

type PanelResizerProps = {
  collapsed: boolean;
  side: "left" | "right";
  onResize: (delta: number) => void;
};

function PanelResizer({ collapsed, onResize }: PanelResizerProps) {
  const lastX = useRef<number | null>(null);

  function handlePointerDown(event: React.PointerEvent<HTMLDivElement>) {
    if (collapsed) return;
    event.preventDefault();
    lastX.current = event.clientX;
    event.currentTarget.setPointerCapture(event.pointerId);
  }

  function handlePointerMove(event: React.PointerEvent<HTMLDivElement>) {
    if (lastX.current === null) return;
    const delta = event.clientX - lastX.current;
    lastX.current = event.clientX;
    onResize(delta);
  }

  function handlePointerEnd(event: React.PointerEvent<HTMLDivElement>) {
    lastX.current = null;
    if (event.currentTarget.hasPointerCapture(event.pointerId)) {
      event.currentTarget.releasePointerCapture(event.pointerId);
    }
  }

  return (
    <div
      aria-hidden="true"
      className="panel-resizer"
      onPointerCancel={handlePointerEnd}
      onPointerDown={handlePointerDown}
      onPointerMove={handlePointerMove}
      onPointerUp={handlePointerEnd}
      role="separator"
    />
  );
}

function newID() {
  return crypto.randomUUID();
}

export function Workbench() {
  // ---- 页面级状态：跨面板事实（当前 Session、Run、模型、布局）。组件内交互状态留在对应 feature。----
  const [sessionID, setSessionID] = useState(newID);
  const [sessions, setSessions] = useState<SessionSummary[]>([]);
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [input, setInput] = useState("");
  const [requestID, setRequestID] = useState<string | null>(null);
  const [isCommandRunning, setIsCommandRunning] = useState(false);
  const [commands, setCommands] = useState<CommandDefinition[]>([]);
  const [mcpServers, setMcpServers] = useState<McpServerStatus[]>([]);
  const [skills, setSkills] = useState<SkillEntry[]>([]);
  const [modelCatalog, setModelCatalog] = useState<ModelCatalog | null>(null);
  const [modelID, setModelID] = useState("");
  const [thinkingMode, setThinkingMode] = useState("");
  const [contextTokens, setContextTokens] = useState<number | null>(null);
  const [pendingImages, setPendingImages] = useState<PendingImage[]>([]);
  const [error, setError] = useState("");
  const [commandStatus, setCommandStatus] = useState("");
  const [isStopping, setIsStopping] = useState(false);
  const [sidebarWidth, setSidebarWidth] = useState(260);
  const [sidebarCollapsed, setSidebarCollapsed] = useState(false);
  const [inspectorWidth, setInspectorWidth] = useState(570);
  const [inspectorCollapsed, setInspectorCollapsed] = useState(false);
  const isRunning = requestID !== null;
  const isBusy = isRunning || isCommandRunning;
  const currentSession = sessions.find((session) => session.id === sessionID);

  // ---- 会话：列表、历史、上下文用量 ----
  async function refreshSessions() {
    try {
      setSessions(await listSessions());
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "无法读取会话列表");
    }
  }

  async function refreshContext(nextSessionID: string) {
    try {
      const usage = await getSessionContext(nextSessionID);
      setContextTokens(usage.promptTokens);
    } catch {
      // 用量读取失败不打断当前会话；保留上一次的展示状态。
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

  // ---- 模型与图片 ----
  function selectModel(nextModelID: string) {
    setModelID(nextModelID);
    const nextModel = modelCatalog?.models.find((model) => model.id === nextModelID);
    setThinkingMode(nextModel?.thinking.defaultMode ?? "");
    if (!nextModel?.vision && pendingImages.length > 0) {
      setPendingImages([]);
      setError("当前模型不支持图片，已移除图片");
    }
  }

  function addImages(files: File[]) {
    const remaining = 5 - pendingImages.length;
    if (remaining <= 0) {
      setError("最多上传 5 张图片");
      return;
    }
    const oversized = files.find((file) => file.size > 10 * 1024 * 1024);
    if (oversized) {
      setError(`图片 ${oversized.name} 超过 10MB，已跳过`);
    }
    for (const file of files.filter((file) => file.size <= 10 * 1024 * 1024).slice(0, remaining)) {
      const reader = new FileReader();
      reader.onload = () => {
        setPendingImages((current) => [...current, { id: newID(), name: file.name, dataUrl: String(reader.result) }]);
      };
      reader.readAsDataURL(file);
    }
  }

  function removeImage(id: string) {
    setPendingImages((current) => current.filter((image) => image.id !== id));
  }

  function newSession() {
    if (isBusy) return;
    setSessionID(newID());
    setMessages([]);
    setContextTokens(null);
    setError("");
  }

  async function selectSession(nextSessionID: string) {
    if (isBusy || nextSessionID === sessionID) return;
    try {
      const history = await getSession(nextSessionID);
      setSessionID(nextSessionID);
      setMessages(history.messages);
      void refreshContext(nextSessionID);
      setError("");
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "无法读取会话历史");
    }
  }

  async function removeSession(targetSessionID: string) {
    if (isBusy || !window.confirm("删除这条会话？此操作无法恢复。")) return;
    try {
      await deleteSession(targetSessionID);
      if (targetSessionID === sessionID) newSession();
      await refreshSessions();
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "无法删除会话");
    }
  }

  useEffect(() => {
    void listCommands()
      .then(setCommands)
      .catch((cause: unknown) => setError(cause instanceof Error ? cause.message : "无法读取命令目录"));
  }, []);

  useEffect(() => {
    void getMcpStatus()
      .then((result) => setMcpServers(result.servers))
      .catch((cause: unknown) => setError(cause instanceof Error ? cause.message : "无法读取 MCP 状态"));
  }, []);

  useEffect(() => {
    void getSkills()
      .then(setSkills)
      .catch((cause: unknown) => setError(cause instanceof Error ? cause.message : "无法读取技能列表"));
  }, []);

  // ---- Run：SSE 事件归并、提交与停止 ----
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
    if (!message || isBusy || !modelCatalog || !modelID || !thinkingMode) {
      return;
    }

    if (message.startsWith("/")) {
      setInput("");
      setError("");
      setCommandStatus("");
      setIsCommandRunning(true);
      try {
        const response = await executeCommand({
          sessionId: sessionID,
          command: message,
          modelId: modelID,
          thinkingMode,
        });
        if (!response.ok) {
          const result = await response.json().catch(() => null) as { message?: string } | null;
          throw new Error(result?.message ?? "命令执行失败");
        }
        setCommandStatus("当前会话上下文已压缩");
        void refreshContext(sessionID);
      } catch (cause) {
        setError(cause instanceof Error ? cause.message : "命令执行失败");
      } finally {
        setIsCommandRunning(false);
        void refreshSessions();
      }
      return;
    }

    const nextRequestID = newID();
    const assistantID = newID();
    const images = pendingImages.map(({ name, dataUrl }) => ({ name, dataUrl }));
    setMessages((current) => [
      ...current,
      { id: newID(), role: "user", content: message, images },
      { id: assistantID, role: "assistant", blocks: [] },
    ]);
    setInput("");
    setPendingImages([]);
    setError("");
    setCommandStatus("");
    setRequestID(nextRequestID);
    try {
      const response = await startRun({
        requestId: nextRequestID,
        sessionId: sessionID,
        message,
        modelId: modelID,
        thinkingMode,
        images,
      });
      if (!response.ok) {
        const result = await response.json().catch(() => null) as { message?: string } | null;
        throw new Error(result?.message ?? "无法启动 Agent 请求");
      }
      await consumeSSE(response, (streamEvent) => applyStreamEvent(assistantID, streamEvent));
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "Agent 请求失败");
    } finally {
      setRequestID(null);
      setIsStopping(false);
      void refreshContext(sessionID);
      void refreshSessions();
    }
  }

  function selectCommand(syntax: string) {
    setInput(syntax);
    setCommandStatus("");
    setError("");
  }

  function selectSkill(name: string) {
    setInput(name);
    setCommandStatus("");
    setError("");
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
      <div className="pane" style={{ width: sidebarCollapsed ? 0 : sidebarWidth }}>
        <SessionSidebar
          sessions={sessions}
          sessionID={sessionID}
          isRunning={isRunning}
          mcps={mcpServers}
          skills={skills}
          onNew={newSession}
          onSelect={selectSession}
          onDelete={removeSession}
          onSelectSkill={selectSkill}
        />
      </div>
      <PanelResizer
        collapsed={sidebarCollapsed}
        side="left"
        onResize={(delta) => setSidebarWidth((width) => clamp(width + delta, SIDEBAR_MIN, SIDEBAR_MAX))}
      />
      <section className="chat-area">
        <header className="chat-header">
          <div className="chat-header-side">
            <button
              aria-label={sidebarCollapsed ? "展开会话列表" : "折叠会话列表"}
              className="panel-toggle"
              onClick={() => setSidebarCollapsed((collapsed) => !collapsed)}
              type="button"
            >
              <Icon name="chevron" className={sidebarCollapsed ? "icon-rotate-right" : "icon-rotate-left"} />
            </button>
            <span className="chat-title">{currentSession?.title ?? "当前临时会话"}</span>
          </div>
          <div className="chat-header-side">
            <RuntimeStatus isRunning={isRunning} requestID={requestID} />
            <button
              aria-label={inspectorCollapsed ? "展开文件与代码" : "折叠文件与代码"}
              className="panel-toggle"
              onClick={() => setInspectorCollapsed((collapsed) => !collapsed)}
              type="button"
            >
              <Icon name="chevron" className={inspectorCollapsed ? "icon-rotate-left" : "icon-rotate-right"} />
            </button>
          </div>
        </header>
        <section className="messages">
          <ChatTimeline isRunning={isRunning} messages={messages} />
        </section>
        {commandStatus && <p className="command-status">{commandStatus}</p>}
        {error && <p className="error">{error}</p>}
        <Composer
          input={input}
          isRunning={isRunning}
          isBusy={isBusy}
          isStopping={isStopping}
          commands={commands}
          modelCatalog={modelCatalog}
          modelID={modelID}
          thinkingMode={thinkingMode}
          contextTokens={contextTokens}
          images={pendingImages}
          onInput={setInput}
          onAddImages={addImages}
          onRemoveImage={removeImage}
          onCommandSelect={selectCommand}
          onModelChange={selectModel}
          onThinkingModeChange={setThinkingMode}
          onStop={stop}
          onSubmit={submit}
        />
      </section>
      <PanelResizer
        collapsed={inspectorCollapsed}
        side="right"
        onResize={(delta) => setInspectorWidth((width) => clamp(width - delta, INSPECTOR_MIN, INSPECTOR_MAX))}
      />
      <div className="pane" style={{ width: inspectorCollapsed ? 0 : inspectorWidth }}>
        <FilesPanel />
      </div>
    </main>
  );
}
