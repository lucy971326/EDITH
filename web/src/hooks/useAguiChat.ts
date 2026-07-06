import { useCallback, useEffect, useRef, useState } from "react";
import { streamAguiSse, type AguiSseEvent } from "../agui/sse";

export type UiMessage = {
  id: string;
  role: "user" | "assistant";
  kind: "text" | "thinking" | "tool-call";
  title?: string;
  content: string;
  status?: "streaming" | "complete" | "error";
  toolCall?: {
    toolCallId: string;
    toolCallName: string;
    args?: string;
    result?: string;
  };
  timestamp: number;
};

function randomId(prefix: string) {
  return `${prefix}_${Date.now()}_${Math.random().toString(16).slice(2)}`;
}

// ── Ref-based message store ──
// 所有频繁更新的数据存在 Ref 里，不触发 React 渲染
// 每帧通过 RAF 同步一次到 State，React 才渲染
class MessageStore {
  messages: UiMessage[] = [];
  indexById = new Map<string, number>();
  toolArgsById = new Map<string, string>();
  toolNameById = new Map<string, string>();
  activeThinkingId: string | null = null;

  /** O(1) 查找消息，找不到返回 -1 */
  indexOf(id: string): number {
    const idx = this.indexById.get(id);
    return idx !== undefined && idx < this.messages.length &&
      this.messages[idx]?.id === id ? idx : -1;
  }

  /** 追加新消息 */
  add(msg: UiMessage) {
    const idx = this.messages.length;
    this.messages.push(msg);
    this.indexById.set(msg.id, idx);
  }

  /** 按 ID 更新消息，找不到则追加 */
  upsert(id: string, updater: (prev: UiMessage | null) => UiMessage) {
    const idx = this.indexOf(id);
    if (idx === -1) {
      const created = updater(null);
      this.add(created);
      return created;
    }
    const updated = updater(this.messages[idx]);
    this.messages[idx] = updated;
    this.indexById.set(updated.id, idx);
    return updated;
  }

  /** 批量替换（用于历史加载） */
  replace(msgs: UiMessage[]) {
    this.messages = [];
    this.indexById.clear();
    for (const msg of msgs) {
      this.add(msg);
    }
  }

  clear() {
    this.messages = [];
    this.indexById.clear();
    this.toolArgsById.clear();
    this.toolNameById.clear();
    this.activeThinkingId = null;
  }

  /** 快照一份给 React */
  snapshot(): UiMessage[] {
    return this.messages.slice();
  }
}

export function useAguiChat(endpoint: string, threadId: string) {
  // State 只用于触发 React 渲染
  const [, forceRender] = useState(0);
  const [inProgress, setInProgress] = useState(false);
  const [lastError, setLastError] = useState<string | null>(null);

  // Refs 存实际数据
  const storeRef = useRef(new MessageStore());
  const abortRef = useRef<AbortController | null>(null);
  const rafIdRef = useRef<number>(0);
  const rafPendingRef = useRef(false);

  // 组件卸载时取消 RAF
  useEffect(() => {
    return () => {
      if (rafIdRef.current) cancelAnimationFrame(rafIdRef.current);
    };
  }, []);

  // RAF 批量渲染：事件来了先存 Ref，每帧只同步一次到 React
  const scheduleRender = useCallback(() => {
    if (rafPendingRef.current) return;
    rafPendingRef.current = true;
    rafIdRef.current = requestAnimationFrame(() => {
      rafPendingRef.current = false;
      forceRender((n) => n + 1); // 触发一次 React 渲染
    });
  }, []);

  const handleEvent = useCallback((evt: AguiSseEvent) => {
    const type = String(evt.type ?? "");
    const store = storeRef.current;

    // ── Lifecycle ──
    if (type === "RUN_STARTED") {
      setInProgress(true);
      setLastError(null);
      return;
    }
    if (type === "RUN_FINISHED") {
      setInProgress(false);
      abortRef.current = null;
      scheduleRender();
      return;
    }
    if (type === "RUN_ERROR") {
      setInProgress(false);
      setLastError(String(evt.message ?? "Run error"));
      abortRef.current = null;
      scheduleRender();
      return;
    }

    // ── Thinking (CUSTOM events) ──
    if (type === "CUSTOM") {
      const name = String(evt.name ?? "");
      const value = evt.value as any;
      if (name === "think_start") {
        const id = randomId("thinking");
        store.activeThinkingId = id;
        store.add({ id, role: "assistant", kind: "thinking", content: "", status: "streaming", timestamp: Date.now() });
        scheduleRender();
        return;
      }
      if (name === "think_content") {
        const chunk = typeof value === "string" ? value : "";
        if (store.activeThinkingId) {
          store.upsert(store.activeThinkingId, (m) => ({ ...m!, content: (m?.content ?? "") + chunk }));
          scheduleRender();
        }
        return;
      }
      if (name === "think_end") {
        if (store.activeThinkingId) {
          store.upsert(store.activeThinkingId, (m) => ({ ...m!, status: "complete" }));
        }
        store.activeThinkingId = null;
        scheduleRender();
        return;
      }
    }

    // ── Reasoning ──
    if (type.startsWith("REASONING_")) {
      const msgId = String(evt.messageId ?? "");
      const delta = String(evt.delta ?? "");
      const reasoningId = `reasoning_${msgId}`;

      if (type === "REASONING_START" || type === "REASONING_MESSAGE_START") {
        store.upsert(reasoningId, () => ({
          id: reasoningId, role: "assistant", kind: "thinking", title: "Thinking",
          content: "", status: "streaming", timestamp: Date.now(),
        }));
        scheduleRender();
        return;
      }
      if (type === "REASONING_MESSAGE_CHUNK" || type === "REASONING_MESSAGE_CONTENT") {
        if (!delta) return;
        store.upsert(reasoningId, (m) => ({
          ...m!, id: reasoningId, role: "assistant", kind: "thinking",
          content: (m?.content ?? "") + delta, status: "streaming",
        }));
        scheduleRender();
        return;
      }
      if (type === "REASONING_MESSAGE_END" || type === "REASONING_END") {
        store.upsert(reasoningId, (m) => ({ ...m!, status: "complete" }));
        scheduleRender();
        return;
      }
    }

    // ── Assistant text ──
    if (type === "TEXT_MESSAGE_START") {
      const msgId = String(evt.messageId ?? randomId("assistant"));
      store.add({ id: msgId, role: "assistant", kind: "text", title: "EDITH", content: "", timestamp: Date.now() });
      scheduleRender();
      return;
    }
    if (type === "TEXT_MESSAGE_CONTENT") {
      const msgId = String(evt.messageId ?? "");
      const delta = String(evt.delta ?? "");
      if (!msgId || !delta) return;
      store.upsert(msgId, (m) => ({ ...m!, content: (m?.content ?? "") + delta }));
      scheduleRender();
      return;
    }

    // ── Tool calls ──
    if (type === "TOOL_CALL_START") {
      const toolCallId = String(evt.toolCallId ?? randomId("tool"));
      const toolName = String(evt.toolCallName ?? "tool");
      store.toolArgsById.set(toolCallId, "");
      store.toolNameById.set(toolCallId, toolName);
      store.add({
        id: toolCallId, role: "assistant", kind: "tool-call",
        title: toolName, content: "", status: "streaming",
        toolCall: { toolCallId, toolCallName: toolName, args: "" },
        timestamp: Date.now(),
      });
      scheduleRender();
      return;
    }
    if (type === "TOOL_CALL_ARGS") {
      const toolCallId = String(evt.toolCallId ?? "");
      const delta = String(evt.delta ?? "");
      if (!toolCallId) return;
      const prev = store.toolArgsById.get(toolCallId) ?? "";
      store.toolArgsById.set(toolCallId, prev + delta);
      store.upsert(toolCallId, (m) => ({
        ...m!, content: prev + delta,
        toolCall: { ...m!.toolCall!, args: prev + delta },
      }));
      scheduleRender();
      return;
    }
    if (type === "TOOL_CALL_RESULT") {
      const toolCallId = String(evt.toolCallId ?? "");
      const content = String(evt.content ?? "");
      if (!toolCallId) return;
      const args = store.toolArgsById.get(toolCallId) ?? "";
      store.upsert(toolCallId, (m) => ({
        ...m!, status: "complete",
        toolCall: { ...m!.toolCall!, args, result: content },
      }));
      scheduleRender();
      return;
    }
  }, [scheduleRender]);

  const send = useCallback(async (text: string) => {
    const trimmed = text.trim();
    if (!trimmed) return;

    storeRef.current.add({
      id: randomId("user"), role: "user", kind: "text", title: "You",
      content: trimmed, timestamp: Date.now(),
    });
    scheduleRender();

    abortRef.current?.abort();
    const controller = new AbortController();
    abortRef.current = controller;
    setInProgress(true);

    try {
      await streamAguiSse(endpoint, {
        threadId, runId: randomId("run"), messages: [{ role: "user", content: trimmed }],
      }, { signal: controller.signal, onEvent: handleEvent });
    } catch (error: any) {
      if (controller.signal.aborted) return;
      setInProgress(false);
      setLastError(String(error?.message ?? error));
    }
  }, [handleEvent, endpoint, threadId, scheduleRender]);

  const loadHistory = useCallback(async (sessionId?: string) => {
    const sid = sessionId ?? threadId;
    abortRef.current?.abort();
    const controller = new AbortController();
    abortRef.current = controller;
    setInProgress(true);

    try {
      const res = await fetch(`/api/history?session_id=${encodeURIComponent(sid)}`, {
        signal: controller.signal,
      });
      if (!res.ok) {
        storeRef.current.clear();
        scheduleRender();
        setInProgress(false);
        return;
      }
      const data = await res.json();
      const uiMessages: UiMessage[] = (Array.isArray(data) ? data : []).map((m: any) => ({
        id: randomId("hist"),
        role: m.role === "user" ? "user" as const : "assistant" as const,
        kind: "text" as const,
        title: m.role === "user" ? "You" : "EDITH",
        content: String(m.content ?? ""),
        timestamp: Date.now(),
      }));
      storeRef.current.replace(uiMessages);
      scheduleRender();
      setInProgress(false);
    } catch {
      setInProgress(false);
    }
  }, [threadId, scheduleRender]);

  const stop = useCallback(() => {
    abortRef.current?.abort();
    abortRef.current = null;
    setInProgress(false);
  }, []);

  const reset = useCallback(() => {
    abortRef.current?.abort();
    abortRef.current = null;
    storeRef.current.clear();
    forceRender((n) => n + 1);
  }, []);

  return {
    messages: storeRef.current.snapshot(),
    inProgress,
    lastError,
    send,
    loadHistory,
    stop,
    reset,
  };
}
