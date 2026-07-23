"use client";

import { useState } from "react";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import type { ChatMessage } from "@/types/api";

type AgentMessage = Extract<ChatMessage, { kind: "assistant" | "reasoning" | "tool" }>;
type RunEntry = AgentMessage | Extract<ChatMessage, { kind: "error" }>;
type ConversationRun = {
  id: string;
  user?: Extract<ChatMessage, { kind: "user" }>;
  entries: RunEntry[];
};

export function Conversation({ messages }: { messages: ChatMessage[] }) {
  const runs = buildRuns(messages);

  if (runs.length === 0) {
    return (
      <div className="flex h-full items-center justify-center text-center text-sm text-slate-400">
        <div>
          <div className="mb-2 text-3xl">👋</div>
          跟小天打个招呼吧
        </div>
      </div>
    );
  }

  return (
    <div className="mx-auto w-full max-w-4xl space-y-5">
      {runs.map((run) => <ConversationRunView key={run.id} run={run} />)}
    </div>
  );
}

function buildRuns(messages: ChatMessage[]): ConversationRun[] {
  const runs: ConversationRun[] = [];
  let current: ConversationRun | undefined;

  for (const message of messages) {
    if (message.kind === "user") {
      current = { id: message.id, user: message, entries: [] };
      runs.push(current);
      continue;
    }
    if (!current) {
      current = { id: `orphan-${message.id}`, entries: [] };
      runs.push(current);
    }
    current.entries.push(message);
  }

  return runs;
}

function UserMessage({ text }: { text: string }) {
  return (
    <div className="flex justify-end">
      <div className="max-w-[78%] rounded-2xl rounded-br-sm bg-blue-600 px-4 py-2.5 text-sm leading-relaxed text-white shadow-sm">
        {text}
      </div>
    </div>
  );
}

function ConversationRunView({ run }: { run: ConversationRun }) {
  const steps = groupSteps(run.entries);
  return (
    <div className="space-y-4">
      {run.user && <UserMessage text={run.user.text} />}
      {steps.length > 0 && (
        <div className="flex items-start gap-3">
          <div className="mt-0.5 flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-blue-100 text-sm font-semibold text-blue-600">A</div>
          <div className="min-w-0 flex-1 space-y-4 pt-0.5">
            {steps.map((step) => step.kind === "error"
              ? <ErrorMessage key={step.message.id} text={step.message.text} />
              : <AgentStep key={step.id} messages={step.messages} />,
            )}
          </div>
        </div>
      )}
    </div>
  );
}

type AgentStepItem = { kind: "step"; id: string; messages: AgentMessage[] } | { kind: "error"; message: Extract<ChatMessage, { kind: "error" }> };

function groupSteps(entries: RunEntry[]): AgentStepItem[] {
  const steps: AgentStepItem[] = [];
  for (const entry of entries) {
    if (entry.kind === "error") {
      steps.push({ kind: "error", message: entry });
      continue;
    }
    const previous = steps[steps.length - 1];
    if (previous?.kind === "step" && previous.id === entry.response_id) {
      previous.messages.push(entry);
    } else {
      steps.push({ kind: "step", id: entry.response_id, messages: [entry] });
    }
  }
  return steps;
}

function AgentStep({ messages }: { messages: AgentMessage[] }) {
  return (
    <div className="space-y-3">
      {messages.map((message) => {
        switch (message.kind) {
          case "reasoning":
            return <Thinking key={message.id} text={message.text} />;
          case "tool":
            return <ToolExecution key={message.id} message={message} />;
          case "assistant":
            return <AssistantText key={message.id} text={message.text} done={message.done} />;
        }
      })}
    </div>
  );
}

function Thinking({ text }: { text: string }) {
  const [open, setOpen] = useState(false);
  return (
    <div>
      <button
        type="button"
        onClick={() => setOpen((value) => !value)}
        className="flex items-center gap-1.5 text-xs text-slate-400 transition hover:text-slate-600"
      >
        <span>{open ? "▾" : "▸"}</span>
        思考过程
      </button>
      {open && (
        <div className="mt-1.5 rounded-xl border border-slate-200 bg-slate-50 px-3 py-2 text-xs leading-6 text-slate-500">
          {text}
        </div>
      )}
    </div>
  );
}

function ToolExecution({ message }: { message: Extract<ChatMessage, { kind: "tool" }> }) {
  return (
    <div className="rounded-xl border border-blue-100 bg-blue-50/60 p-3 text-xs">
      <div className="flex items-center gap-2 text-blue-700">
        <span>🔧</span>
        <span className="font-mono font-medium">{message.name}</span>
        {message.result === undefined && <span className="text-blue-400">执行中…</span>}
        {message.result !== undefined && <span className="text-emerald-600">已完成</span>}
      </div>
      <pre className="mt-2 overflow-x-auto whitespace-pre-wrap rounded-lg bg-white/70 p-2 font-mono text-[11px] leading-5 text-slate-600">
        {formatArguments(message.arguments)}
      </pre>
      {message.result !== undefined && (
        <pre className="mt-2 max-h-64 overflow-auto whitespace-pre-wrap rounded-lg border border-emerald-100 bg-emerald-50 p-2 font-mono text-[11px] leading-5 text-emerald-800">
          {JSON.stringify(message.result, null, 2)}
        </pre>
      )}
    </div>
  );
}

function AssistantText({ text, done }: { text: string; done: boolean }) {
  if (!text && done) return null;
  return (
    <div className="rounded-2xl rounded-tl-sm bg-slate-100 px-4 py-3 text-sm leading-7 text-slate-800">
      <ReactMarkdown remarkPlugins={[remarkGfm]} components={markdownComponents}>
        {text}
      </ReactMarkdown>
      {!done && <span className="ml-1 inline-block h-4 w-1 animate-pulse bg-blue-600 align-[-2px]" />}
    </div>
  );
}

function ErrorMessage({ text }: { text: string }) {
  return <div className="rounded-xl bg-red-50 px-3 py-2 text-center text-xs text-red-600">{text}</div>;
}

function formatArguments(raw: string): string {
  try {
    return JSON.stringify(JSON.parse(raw), null, 2);
  } catch {
    return raw;
  }
}

const markdownComponents = {
  pre({ children }: { children?: React.ReactNode }) {
    return <pre className="my-2 overflow-x-auto rounded-lg bg-slate-800 p-3 text-xs text-slate-100">{children}</pre>;
  },
  code({ children, className, ...rest }: React.ComponentPropsWithoutRef<"code"> & { node?: unknown }) {
    if (className?.startsWith("language-")) return <code className={`${className} text-xs`} {...rest}>{children}</code>;
    return <code className="rounded bg-slate-200 px-1 py-0.5 font-mono text-xs text-slate-800" {...rest}>{children}</code>;
  },
  p({ children }: { children?: React.ReactNode }) {
    return <p className="mb-2 last:mb-0">{children}</p>;
  },
  ul({ children }: { children?: React.ReactNode }) {
    return <ul className="mb-2 list-disc pl-5 last:mb-0">{children}</ul>;
  },
  ol({ children }: { children?: React.ReactNode }) {
    return <ol className="mb-2 list-decimal pl-5 last:mb-0">{children}</ol>;
  },
  a({ href, children }: { href?: string; children?: React.ReactNode }) {
    return <a href={href} target="_blank" rel="noreferrer" className="text-blue-600 underline">{children}</a>;
  },
};
