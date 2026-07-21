"use client";

import { useState, useRef, useEffect } from "react";
import { fetchEventSource } from "@microsoft/fetch-event-source";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import type {
  AgentEvent,
  StreamRequest,
  ModelInfo,
  ImageInput,
} from "@/types/api";

// ---------------------------------------------------------------------------
// 消息模型
// ---------------------------------------------------------------------------

type Message =
  | { id: string; kind: "user"; text: string }
  | { id: string; kind: "assistant"; text: string; done: boolean }
  | {
      id: string;
      kind: "tool";
      name: string;
      arguments: string;
      result?: unknown;
    }
  | { id: string; kind: "reasoning"; text: string }
  | { id: string; kind: "error"; text: string };

// ---------------------------------------------------------------------------
// Main
// ---------------------------------------------------------------------------

export default function ChatPage() {
  const [messages, setMessages] = useState<Message[]>([]);
  const [input, setInput] = useState("");
  const [streaming, setStreaming] = useState(false);
  const bottomRef = useRef<HTMLDivElement>(null);

  // 模型选择
  const [models, setModels] = useState<ModelInfo[]>([]);
  const [model, setModel] = useState("");
  const currentModel = models.find((m) => m.id === model);

  useEffect(() => {
    fetch("/api/models")
      .then((r) => r.json())
      .then((list: ModelInfo[]) => {
        setModels(list);
        if (list.length > 0) setModel(list[0].id);
      })
      .catch(() => {});
  }, []);

  // 图片上传
  const [images, setImages] = useState<
    { data: string; format: string; preview: string }[]
  >([]);
  const fileRef = useRef<HTMLInputElement>(null);

  function addImage(file: File) {
    if (!file.type.startsWith("image/")) return;
    const format = file.type.split("/")[1]; // "png" | "jpeg" | "webp" | "gif"
    const reader = new FileReader();
    reader.onload = () => {
      const dataUrl = reader.result as string;
      const base64 = dataUrl.split(",")[1]; // 去掉 "data:image/png;base64," 前缀
      setImages((imgs) => [
        ...imgs,
        { data: base64, format, preview: dataUrl },
      ]);
    };
    reader.readAsDataURL(file);
  }

  function removeImage(i: number) {
    setImages((imgs) => imgs.filter((_, idx) => idx !== i));
  }

  // 自动滚到底部
  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [messages]);

  async function send() {
    if (!input.trim() || streaming) return;
    const userText = input;
    const sentImages = [...images];
    setInput("");
    setImages([]);
    setStreaming(true);

    // 用户消息 + 空的 assistant 占位
    const userMsg: Message = {
      id: crypto.randomUUID(),
      kind: "user",
      text: userText,
    };
    const asstMsg: Message = {
      id: crypto.randomUUID(),
      kind: "assistant",
      text: "",
      done: false,
    };
    setMessages((m) => [...m, userMsg, asstMsg]);

    const body: StreamRequest = {
      user_id: "u-alice",
      session_id: "002",
      message: userText,
      model,
    };
    if (sentImages.length > 0) {
      body.images = sentImages.map(({ data, format }) => ({ data, format }));
    }

    const ctrl = new AbortController();
    await fetchEventSource("/api/stream", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
      signal: ctrl.signal,

      onmessage(event) {
        const data = JSON.parse(event.data) as AgentEvent;
        setMessages((m) => updateMessages(m, data));
        if (data.type === "done" || data.type === "error") {
          // 停止光标闪烁
          setMessages((m) => finishLastAssistant(m));
          if (data.type === "done") {
            ctrl.abort();
            setStreaming(false);
          }
        }
      },

      onerror(err) {
        setMessages((m) =>
          finishLastAssistant([
            ...m,
            { id: crypto.randomUUID(), kind: "error", text: String(err) },
          ]),
        );
        setStreaming(false);
        ctrl.abort();
      },
    });
  }

  return (
    <div className="flex h-screen flex-col bg-white">
      {/* 顶栏 */}
      <header className="flex items-center gap-3 border-b px-6 py-3">
        <div className="flex h-9 w-9 items-center justify-center rounded-full bg-blue-600 text-white font-bold text-sm">
          A
        </div>
        <div>
          <h1 className="text-lg font-semibold leading-tight">小天</h1>
          <p className="text-xs text-gray-500">
            {streaming ? "正在输入…" : "在线"}
          </p>
        </div>
        <div className="ml-auto flex items-center gap-3">
          {models.length > 1 && (
            <select
              value={model}
              onChange={(e) => setModel(e.target.value)}
              className="rounded border border-gray-200 bg-gray-50 px-2 py-1 text-xs text-gray-600 outline-none focus:border-blue-400"
            >
              {models.map((m) => (
                <option key={m.id} value={m.id}>
                  {m.label}
                </option>
              ))}
            </select>
          )}
          <span className="text-xs text-gray-400">Session: 002</span>
        </div>
      </header>

      {/* 消息区 */}
      <div className="flex-1 overflow-y-auto px-6 py-6">
        {messages.length === 0 && (
          <div className="flex h-full items-center justify-center text-gray-400">
            <div className="text-center">
              <div className="mb-2 text-4xl">👋</div>
              <div>跟小天打个招呼吧</div>
            </div>
          </div>
        )}

        {messages.map((m) => (
          <MessageBubble key={m.id} msg={m} />
        ))}

        {/* 滚动锚点 */}
        <div ref={bottomRef} />
      </div>

      {/* 输入栏 */}
      <footer
        className="border-t px-6 py-3"
        onDragOver={(e) => {
          e.preventDefault();
        }}
        onDrop={(e) => {
          e.preventDefault();
          Array.from(e.dataTransfer.files).forEach(addImage);
        }}
      >
        {/* 图片预览 */}
        {images.length > 0 && (
          <div className="mx-auto mb-3 flex max-w-3xl flex-wrap gap-2">
            {images.map((img, i) => (
              <div key={i} className="relative group">
                <img
                  src={img.preview}
                  className="h-16 w-16 rounded-lg border object-cover"
                />
                <button
                  onClick={() => removeImage(i)}
                  className="absolute -right-1.5 -top-1.5 flex h-5 w-5 items-center justify-center rounded-full bg-gray-700 text-[10px] text-white opacity-0 transition group-hover:opacity-100"
                >
                  ✕
                </button>
              </div>
            ))}
          </div>
        )}

        <div className="mx-auto flex max-w-3xl gap-2">
          {/* 上传按钮 — 仅视觉模型显示 */}
          {currentModel?.vision && (
            <>
              <input
                ref={fileRef}
                type="file"
                accept="image/*"
                multiple
                className="hidden"
                onChange={(e) => {
                  Array.from(e.target.files ?? []).forEach(addImage);
                  e.target.value = "";
                }}
              />
              <button
                onClick={() => fileRef.current?.click()}
                disabled={streaming}
                title="上传图片"
                className="rounded-lg border bg-gray-50 px-3 py-3 text-sm text-gray-500 transition hover:bg-gray-100 disabled:opacity-50"
              >
                🖼
              </button>
            </>
          )}
          <input
            value={input}
            onChange={(e) => setInput(e.target.value)}
            onKeyDown={(e) => e.key === "Enter" && send()}
            onPaste={(e) => {
              const files = e.clipboardData.files;
              if (files.length > 0) {
                e.preventDefault();
                Array.from(files).forEach(addImage);
              }
            }}
            disabled={streaming}
            placeholder={
              currentModel?.vision ? "输入消息，支持粘贴图片…" : "输入消息…"
            }
            className="flex-1 rounded-lg border bg-gray-50 px-4 py-3 text-sm outline-none transition focus:border-blue-400 focus:bg-white focus:ring-2 focus:ring-blue-100 disabled:opacity-50"
          />
          <button
            onClick={send}
            disabled={streaming || (!input.trim() && images.length === 0)}
            className="rounded-lg bg-blue-600 px-5 py-3 text-sm font-medium text-white transition hover:bg-blue-700 disabled:opacity-40"
          >
            发送
          </button>
        </div>
      </footer>
    </div>
  );
}

// ---------------------------------------------------------------------------
// 单条消息气泡
// ---------------------------------------------------------------------------

function MessageBubble({ msg }: { msg: Message }) {
  switch (msg.kind) {
    case "user":
      return <UserBubble text={msg.text} />;
    case "assistant":
      return <AssistantBubble text={msg.text} done={msg.done} />;
    case "reasoning":
      return <ThinkingBubble text={msg.text} />;
    case "tool":
      return (
        <ToolCard name={msg.name} args={msg.arguments} result={msg.result} />
      );
    case "error":
      return <ErrorBubble text={msg.text} />;
  }
}

// ---------------------------------------------------------------------------
// 子组件
// ---------------------------------------------------------------------------

function UserBubble({ text }: { text: string }) {
  return (
    <div className="mb-4 flex justify-end">
      <div className="max-w-[80%] rounded-xl rounded-br-sm bg-blue-600 px-4 py-2.5 text-sm text-white">
        {text}
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Markdown 组件覆盖 —— 聊天风格的样式
// ---------------------------------------------------------------------------

const markdownComponents = {
  pre({ children }: { children?: React.ReactNode }) {
    return (
      <pre className="my-2 overflow-x-auto rounded-lg bg-gray-800 p-3 text-xs text-gray-100">
        {children}
      </pre>
    )
  },
  code({ children, className, ...rest }: React.ComponentPropsWithoutRef<'code'> & { node?: unknown }) {
    const isBlock = className?.startsWith('language-')
    if (isBlock) {
      return (
        <code className={`${className} text-xs leading-relaxed`} {...rest}>
          {children}
        </code>
      )
    }
    return (
      <code className="rounded bg-gray-200 px-1 py-0.5 text-xs font-mono text-gray-800" {...rest}>
        {children}
      </code>
    )
  },
  p({ children }: { children?: React.ReactNode }) {
    return <p className="mb-1.5 last:mb-0">{children}</p>
  },
  ul({ children }: { children?: React.ReactNode }) {
    return <ul className="mb-1.5 list-disc pl-5 last:mb-0">{children}</ul>
  },
  ol({ children }: { children?: React.ReactNode }) {
    return <ol className="mb-1.5 list-decimal pl-5 last:mb-0">{children}</ol>
  },
  a({ href, children }: { href?: string; children?: React.ReactNode }) {
    return (
      <a href={href} target="_blank" rel="noopener noreferrer" className="text-blue-600 underline">
        {children}
      </a>
    )
  },
  table({ children }: { children?: React.ReactNode }) {
    return <table className="mb-1.5 min-w-full border-collapse border border-gray-300 text-xs">{children}</table>
  },
  th({ children }: { children?: React.ReactNode }) {
    return <th className="border border-gray-300 bg-gray-100 px-2 py-1 text-left font-semibold">{children}</th>
  },
  td({ children }: { children?: React.ReactNode }) {
    return <td className="border border-gray-300 px-2 py-1">{children}</td>
  },
}

function AssistantBubble({ text, done }: { text: string; done: boolean }) {
  if (!text && !done) return null
  return (
    <div className="mb-4 flex gap-3">
      <div className="flex h-7 w-7 shrink-0 items-center justify-center rounded-full bg-blue-100 text-xs font-bold text-blue-600">
        A
      </div>
      <div className="max-w-[80%] rounded-xl rounded-bl-sm bg-gray-100 px-4 py-2.5 text-sm leading-relaxed text-gray-800">
        <ReactMarkdown remarkPlugins={[remarkGfm]} components={markdownComponents}>
          {text}
        </ReactMarkdown>
        {!done && (
          <span className="ml-0.5 inline-block h-4 w-1 animate-pulse bg-blue-600" />
        )}
      </div>
    </div>
  )
}

function ThinkingBubble({ text }: { text: string }) {
  const [open, setOpen] = useState(false);
  return (
    <div className="mb-3 ml-10">
      <button
        onClick={() => setOpen(!open)}
        className="flex items-center gap-1.5 text-xs text-gray-400 hover:text-gray-600 transition"
      >
        <span>{open ? "▾" : "▸"}</span> 思考中
      </button>
      {open && (
        <div className="mt-1 rounded-lg border border-gray-200 bg-gray-50 px-3 py-2 text-xs leading-relaxed text-gray-500">
          {text}
        </div>
      )}
    </div>
  );
}

function ToolCard({
  name,
  args,
  result,
}: {
  name: string;
  args: string;
  result?: unknown;
}) {
  return (
    <div className="mb-3 ml-10">
      <div className="inline-flex items-center gap-2 rounded-lg border border-blue-200 bg-blue-50 px-3 py-1.5 text-xs">
        <span className="text-blue-500">🔧</span>
        <span className="font-mono font-medium text-blue-700">{name}</span>
        <span className="text-blue-400">
          (
          <ParsedArgs raw={args} />)
        </span>
        {result === undefined && (
          <span className="ml-1 inline-block h-3 w-3 animate-spin rounded-full border-2 border-blue-300 border-t-blue-600" />
        )}
      </div>
      {result !== undefined && (
        <div className="mt-1 rounded-lg border border-green-200 bg-green-50 px-3 py-1.5 text-xs text-green-800">
          <pre className="whitespace-pre-wrap font-mono text-[11px]">
            {JSON.stringify(result, null, 2)}
          </pre>
        </div>
      )}
    </div>
  );
}

function ErrorBubble({ text }: { text: string }) {
  return (
    <div className="mb-4 flex justify-center">
      <div className="rounded-lg bg-red-50 px-4 py-2 text-xs text-red-600">
        {text}
      </div>
    </div>
  );
}

// 解析 tool call 参数（截断展示）
function ParsedArgs({ raw }: { raw: string }) {
  let text = raw
  try {
    const obj = JSON.parse(raw)
    text = Object.entries(obj).map(
      ([k, v]) => `${k}: ${JSON.stringify(v)}`,
    ).join(", ")
  } catch { /* 不是 JSON 就原样显示 */ }
  return <span>{text}</span>
}

// ---------------------------------------------------------------------------
// 事件 → 消息更新
// ---------------------------------------------------------------------------

function finishLastAssistant(messages: Message[]): Message[] {
  const out = [...messages];
  for (let i = out.length - 1; i >= 0; i--) {
    const m = out[i];
    if (m.kind === "assistant") {
      out[i] = { ...m, done: true };
      break;
    }
  }
  return out;
}

function updateMessages(messages: Message[], data: AgentEvent): Message[] {
  const out = [...messages];

  switch (data.type) {
    case "text":
      for (let i = out.length - 1; i >= 0; i--) {
        const m = out[i];
        if (m.kind === "assistant") {
          out[i] = { ...m, text: m.text + (data.text ?? "") } as Message;
          break;
        }
      }
      break;
    case "reasoning":
      // 拼到上一条 reasoning（流式 chunk），否则新建
      {
        const last = out[out.length - 1];
        if (last?.kind === "reasoning") {
          out[out.length - 1] = {
            ...last,
            text: last.text + (data.thinking ?? ""),
          } as Message;
        } else {
          out.push({
            id: crypto.randomUUID(),
            kind: "reasoning",
            text: data.thinking ?? "",
          });
        }
      }
      break;
    case "tool_call":
      out.push({
        id: crypto.randomUUID(),
        kind: "tool",
        name: data.name ?? "",
        arguments: data.arguments ?? "",
        _toolId: data.id,
      } as any);
      break;
    case "tool_result":
      // 1) 按 ToolID 精确匹配；2) 兜底：按名称匹配最后一个未完成的
      for (let i = out.length - 1; i >= 0; i--) {
        if (out[i].kind === "tool" && (out[i] as any)._toolId === data.id) {
          out[i] = { ...out[i], result: data.result } as Message;
          return out;
        }
      }
      for (let i = out.length - 1; i >= 0; i--) {
        const item = out[i]
        if (item.kind === "tool") {
          if (item.name === data.name && item.result === undefined) {
            out[i] = { ...item, result: data.result }
            break
          }
        }
      }
      break;
    case "error":
      out.push({
        id: crypto.randomUUID(),
        kind: "error",
        text: data.message ?? "",
      });
      break;
    case "done":
      for (let i = out.length - 1; i >= 0; i--) {
        if (out[i].kind === "assistant") {
          out[i] = { ...out[i], done: true } as Message;
          break;
        }
      }
      break;
  }
  return out;
}
