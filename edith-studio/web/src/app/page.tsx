"use client";

import { FormEvent, useState } from "react";

import { AssistantBlock, readSSEFrames, StreamEvent, ToolStatus } from "../lib/stream";

const agentAPI = "http://127.0.0.1:8765";

type UserMessage = { id: string; role: "user"; content: string };
type AssistantMessage = { id: string; role: "assistant"; blocks: AssistantBlock[] };
type ChatMessage = UserMessage | AssistantMessage;

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

function toolStatus(status: ToolStatus) {
  return { requested: "准备执行", running: "执行中", completed: "完成", failed: "失败" }[status];
}

function BlockView({ block }: { block: AssistantBlock }) {
  if (block.type === "text") {
    return <p className="whitespace-pre-wrap leading-7 text-slate-800">{block.content}</p>;
  }
  if (block.type === "reasoning") {
    return (
      <details className="rounded-xl border border-amber-100 bg-amber-50/70 px-3 py-2 text-sm text-amber-950" open>
        <summary className="cursor-pointer font-medium text-amber-800">思考过程</summary>
        <p className="mt-2 whitespace-pre-wrap leading-6">{block.content}</p>
      </details>
    );
  }
  return (
    <details className="rounded-xl border border-slate-200 bg-slate-50 px-3 py-2 text-sm" open={block.status === "running"}>
      <summary className="cursor-pointer text-slate-700">
        <span className="font-medium">{block.name}</span>
        <span className="ml-2 text-slate-400">{toolStatus(block.status)}</span>
      </summary>
      {block.arguments && <pre className="mt-2 overflow-x-auto whitespace-pre-wrap rounded-lg bg-white p-2 text-xs text-slate-600">{block.arguments}</pre>}
      {block.result && <pre className="mt-2 overflow-x-auto whitespace-pre-wrap rounded-lg bg-white p-2 text-xs text-slate-600">{block.result}</pre>}
    </details>
  );
}

export default function Home() {
  const [sessionID] = useState(newID);
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [input, setInput] = useState("");
  const [requestID, setRequestID] = useState<string | null>(null);
  const [error, setError] = useState("");
  const [isStopping, setIsStopping] = useState(false);
  const isRunning = requestID !== null;

  function applyStreamEvent(assistantID: string, streamEvent: StreamEvent) {
    if (streamEvent.type === "run.error") {
      setError(streamEvent.error?.message ?? "Agent 请求失败");
    }
    setMessages((current) =>
      current.map((message) => (message.id === assistantID && message.role === "assistant" ? applyEvent(message, streamEvent) : message)),
    );
  }

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const message = input.trim();
    if (!message || isRunning) {
      return;
    }
    const nextRequestID = newID();
    const assistantID = newID();
    setMessages((current) => [...current, { id: newID(), role: "user", content: message }, { id: assistantID, role: "assistant", blocks: [] }]);
    setInput("");
    setError("");
    setRequestID(nextRequestID);

    try {
      const response = await fetch(`${agentAPI}/api/runs`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ requestId: nextRequestID, sessionId: sessionID, message }),
      });
      if (!response.ok || !response.body) {
        const result = (await response.json().catch(() => null)) as { message?: string } | null;
        throw new Error(result?.message ?? "无法启动 Agent 请求");
      }
      const reader = response.body.getReader();
      const textDecoder = new TextDecoder();
      let buffer = "";
      while (true) {
        const { done, value } = await reader.read();
        if (done) {
          break;
        }
        buffer += textDecoder.decode(value, { stream: true });
        const frames = readSSEFrames(buffer);
        buffer = frames.rest;
        frames.events.forEach((streamEvent) => applyStreamEvent(assistantID, streamEvent));
      }
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "Agent 请求失败");
    } finally {
      setRequestID(null);
      setIsStopping(false);
    }
  }

  async function stop() {
    if (!requestID || isStopping) {
      return;
    }
    setIsStopping(true);
    try {
      const response = await fetch(`${agentAPI}/api/runs/${requestID}/cancel`, { method: "POST" });
      if (!response.ok) {
        throw new Error("停止请求失败");
      }
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "停止请求失败");
      setIsStopping(false);
    }
  }

  return (
    <main className="mx-auto flex min-h-screen w-full max-w-4xl flex-col px-5 py-8 sm:px-8">
      <header className="mb-8">
        <p className="text-sm font-semibold tracking-[0.22em] text-sky-700">EDITH STUDIO</p>
        <h1 className="mt-2 text-3xl font-semibold">本地 Agent</h1>
        <p className="mt-2 text-sm text-slate-500">单会话 · 实时流式输出</p>
      </header>
      <section className="flex flex-1 flex-col gap-4 rounded-2xl border border-slate-200 bg-white p-5 shadow-sm">
        <div className="min-h-80 space-y-5">
          {messages.length === 0 && <p className="pt-24 text-center text-slate-400">开始和 EDITH 聊聊吧。</p>}
          {messages.map((message) => (
            <article key={message.id} className={message.role === "user" ? "ml-auto max-w-[80%]" : "max-w-[90%]"}>
              <p className="mb-1 text-xs font-medium text-slate-400">{message.role === "user" ? "你" : "EDITH"}</p>
              {message.role === "user" ? (
                <div className="whitespace-pre-wrap rounded-2xl bg-sky-600 px-4 py-3 text-white">{message.content}</div>
              ) : (
                <div className="space-y-3">
                  {message.blocks.map((block) => <BlockView key={block.id} block={block} />)}
                  {message.blocks.length === 0 && isRunning && <p className="text-slate-400">正在思考…</p>}
                </div>
              )}
            </article>
          ))}
        </div>
        {error && <p className="rounded-lg bg-rose-50 px-3 py-2 text-sm text-rose-700">{error}</p>}
        <form onSubmit={submit} className="mt-auto flex gap-3 border-t border-slate-100 pt-4">
          <textarea value={input} onChange={(event) => setInput(event.target.value)} placeholder="输入一句话…" rows={2} disabled={isRunning} className="min-h-12 flex-1 resize-none rounded-xl border border-slate-300 px-3 py-2 outline-none transition focus:border-sky-500 disabled:bg-slate-100" />
          {isRunning ? (
            <button type="button" onClick={stop} disabled={isStopping} className="self-end rounded-xl border border-rose-200 px-4 py-2 text-sm font-medium text-rose-700 disabled:opacity-50">{isStopping ? "停止中…" : "停止"}</button>
          ) : (
            <button type="submit" disabled={!input.trim()} className="self-end rounded-xl bg-sky-600 px-4 py-2 text-sm font-medium text-white disabled:cursor-not-allowed disabled:opacity-50">发送</button>
          )}
        </form>
      </section>
    </main>
  );
}
