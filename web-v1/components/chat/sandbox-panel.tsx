"use client";

import { ChevronDown, ChevronRight, Download, File, Folder, LoaderCircle, RefreshCw, X } from "lucide-react";
import { useEffect, useState, type PointerEvent as ReactPointerEvent } from "react";

import type { SandboxFileContentResponse, SandboxFileEntry, SandboxFilesResponse } from "@/lib/sandbox/type";

type DirectoryState = {
  entries?: SandboxFileEntry[];
  loading?: boolean;
  error?: string;
};

type SandboxRequestError = Error & { code?: string };

type Preview =
  | { type: "text"; file: SandboxFileContentResponse }
  | { type: "binary"; path: string };

function errorMessage(error: unknown) {
  return error instanceof Error ? error.message : "无法读取 Sandbox 文件";
}

async function readJSON<T>(url: string): Promise<T> {
  const response = await fetch(url);
  if (!response.ok) {
    const body = await response.json().catch(() => null) as { type?: string; message?: string } | null;
    const error = new Error(body?.message || "请求失败，请稍后重试") as SandboxRequestError;
    error.code = body?.type;
    throw error;
  }
  return response.json() as Promise<T>;
}

export function SandboxPanel({ sessionID, open }: { sessionID: string; open: boolean }) {
  const [directories, setDirectories] = useState<Record<string, DirectoryState>>({});
  const [expanded, setExpanded] = useState(() => new Set<string>());
  const [panelWidth, setPanelWidth] = useState(320);
  const [preview, setPreview] = useState<Preview | null>(null);
  const [previewError, setPreviewError] = useState<string | null>(null);
  const [isPreviewLoading, setIsPreviewLoading] = useState(false);

  async function loadDirectory(path: string, force = false) {
    if (!force && (directories[path]?.loading || directories[path]?.entries)) return;
    setDirectories((current) => ({ ...current, [path]: { ...current[path], loading: true, error: undefined } }));
    try {
      const query = new URLSearchParams({ sessionId: sessionID });
      if (path) query.set("path", path);
      const response = await readJSON<SandboxFilesResponse>(`/api/sandbox/files?${query}`);
      setDirectories((current) => ({ ...current, [path]: { entries: response.entries } }));
    } catch (error) {
      if ((error as SandboxRequestError).code === "sandbox_not_found") {
        // 未使用过工具的会话没有 Sandbox，这是正常空状态，不应提示重试。
        setDirectories((current) => ({ ...current, [path]: { entries: [] } }));
        return;
      }
      setDirectories((current) => ({ ...current, [path]: { error: errorMessage(error) } }));
    }
  }

  // 会话切换时清除上一会话的缓存与预览，防止短暂展示其他会话的文件。
  useEffect(() => {
    setDirectories({});
    setExpanded(new Set());
    setPreview(null);
    setPreviewError(null);
  }, [sessionID]);

  useEffect(() => {
    if (open) void loadDirectory("");
    // 仅在面板打开时首次加载根目录；目录内容由用户操作按需加载。
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, sessionID]);

  useEffect(() => {
    function refresh() {
      setDirectories({});
      if (open) void loadDirectory("", true);
    }
    window.addEventListener("sandbox-files-updated", refresh);
    return () => window.removeEventListener("sandbox-files-updated", refresh);
    // 事件仅由同一页面的上传成功触发，读取当前 session 的根目录即可。
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, sessionID]);

  async function toggleDirectory(entry: SandboxFileEntry) {
    const next = new Set(expanded);
    if (next.has(entry.path)) {
      next.delete(entry.path);
    } else {
      next.add(entry.path);
      void loadDirectory(entry.path);
    }
    setExpanded(next);
  }

  async function openPreview(entry: SandboxFileEntry) {
    setPreview(null);
    setPreviewError(null);
    setIsPreviewLoading(true);
    try {
      const query = new URLSearchParams({ sessionId: sessionID, path: entry.path });
      setPreview({ type: "text", file: await readJSON<SandboxFileContentResponse>(`/api/sandbox/files/content?${query}`) });
    } catch (error) {
      // 后端不返回二进制字节，避免浏览器内存和安全边界被不必要地扩大。
      if ((error as SandboxRequestError).code === "file_not_previewable") {
        setPreview({ type: "binary", path: entry.path });
      } else {
        setPreviewError(errorMessage(error));
      }
    } finally {
      setIsPreviewLoading(false);
    }
  }

  // 用户可拖动聊天区与文件面板的边界，在可读性范围内调整两者占比。
  function beginResize(event: ReactPointerEvent<HTMLDivElement>) {
    event.preventDefault();
    const startX = event.clientX;
    const startWidth = panelWidth;

    function resize(moveEvent: globalThis.PointerEvent) {
      const nextWidth = startWidth + startX - moveEvent.clientX;
      setPanelWidth(Math.min(640, Math.max(280, nextWidth)));
    }

    function finishResize() {
      window.removeEventListener("pointermove", resize);
      window.removeEventListener("pointerup", finishResize);
    }

    window.addEventListener("pointermove", resize);
    window.addEventListener("pointerup", finishResize);
  }

  function renderEntries(path: string, depth = 0): React.ReactNode {
    const state = directories[path];
    if (state?.loading) return <p className="px-3 py-2 text-xs text-zinc-500"><LoaderCircle className="mr-1 inline size-3 animate-spin" />加载中</p>;
    if (state?.error) return <button className="mx-3 my-2 flex items-center gap-1 text-left text-xs text-red-600 hover:text-red-700" onClick={() => void loadDirectory(path, true)}><RefreshCw className="size-3" />{state.error}，重试</button>;
    return state?.entries?.map((entry) => {
      const isDirectory = entry.type === "directory";
      const isOpen = expanded.has(entry.path);
      const canDownload = !isDirectory && entry.path.startsWith("artifacts/");
      return <div key={entry.path}>
        <div
          className="flex w-full items-center gap-1 rounded px-2 py-1.5 text-left text-sm text-zinc-700 hover:bg-zinc-100"
          style={{ paddingLeft: `${8 + depth * 16}px` }}
        >
          <button className="flex min-w-0 flex-1 items-center gap-1.5 text-left" onClick={() => isDirectory ? void toggleDirectory(entry) : void openPreview(entry)} title={entry.path}>
          {isDirectory ? (isOpen ? <ChevronDown className="size-4 shrink-0" /> : <ChevronRight className="size-4 shrink-0" />) : <span className="w-4 shrink-0" />}
          {isDirectory ? <Folder className="size-4 shrink-0 text-amber-500" /> : <File className="size-4 shrink-0 text-zinc-400" />}
          <span className="truncate">{entry.name}{entry.path === "uploads" && <span className="ml-1 text-xs text-zinc-400">用户上传</span>}</span>
          </button>
          {canDownload && <a aria-label={`下载 ${entry.name}`} className="rounded p-1 text-zinc-500 hover:bg-zinc-200 hover:text-zinc-900" href={`/api/sandbox/files/download?${new URLSearchParams({ sessionId: sessionID, path: entry.path })}`} title="下载交付文件"><Download className="size-3.5" /></a>}
        </div>
        {isDirectory && isOpen && renderEntries(entry.path, depth + 1)}
      </div>;
    });
  }

  if (!open) return null;
  const root = directories[""];
  return <aside className="relative flex shrink-0 flex-col border-l border-zinc-200 bg-white" aria-label="Sandbox 文件" style={{ width: panelWidth }}>
    <div
      aria-label="调整 Sandbox 文件面板宽度"
      className="absolute inset-y-0 -left-1 z-10 w-2 cursor-col-resize touch-none hover:bg-zinc-300"
      onPointerDown={beginResize}
    />
    <div className="flex h-16 items-center justify-between border-b border-zinc-200 px-4">
      <div><p className="text-sm font-medium">Sandbox 文件</p><p className="mt-0.5 text-xs text-zinc-500">当前会话工作区</p></div>
      <button className="rounded p-1.5 text-zinc-500 hover:bg-zinc-100 hover:text-zinc-800" onClick={() => void loadDirectory("", true)} title="刷新根目录"><RefreshCw className="size-4" /></button>
    </div>
    {(isPreviewLoading || preview || previewError) ? <div className="min-h-0 flex flex-1 flex-col">
      <div className="flex h-12 shrink-0 items-center justify-between border-b border-zinc-200 px-3">
        <button className="text-sm text-zinc-600 hover:text-zinc-900" onClick={() => { setPreview(null); setPreviewError(null); setIsPreviewLoading(false); }}>返回文件树</button>
        <p className="mx-4 truncate text-sm font-medium">{preview?.type === "text" ? preview.file.path : preview?.path || "文件预览"}</p>
        <button className="rounded p-1.5 text-zinc-500 hover:bg-zinc-100" onClick={() => { setPreview(null); setPreviewError(null); setIsPreviewLoading(false); }} title="关闭预览"><X className="size-4" /></button>
      </div>
      <div className="min-h-0 flex-1 overflow-auto bg-zinc-50 p-3">
        {isPreviewLoading && <div className="flex h-full items-center justify-center gap-2 text-sm text-zinc-500"><LoaderCircle className="size-4 animate-spin" />正在读取文件</div>}
        {previewError && <div className="py-12 text-center text-sm text-zinc-500"><p>{previewError}</p></div>}
        {preview?.type === "binary" && <div className="py-12 text-center text-sm text-zinc-500">这是二进制文件，暂不支持预览。</div>}
        {preview?.type === "text" && <><pre className="overflow-x-auto rounded-lg border border-zinc-200 bg-white p-3 font-mono text-xs leading-5 text-zinc-800 whitespace-pre-wrap">{preview.file.content}</pre>{preview.file.truncated && <p className="mt-3 text-xs text-amber-700">内容过长，已截断显示。</p>}</>}
      </div>
    </div> : <div className="min-h-0 flex-1 overflow-y-auto p-2">
      {root?.loading && <div className="flex items-center justify-center gap-2 py-8 text-sm text-zinc-500"><LoaderCircle className="size-4 animate-spin" />正在读取文件</div>}
      {root?.error && <div className="px-3 py-8 text-center text-sm text-zinc-500"><p>{root.error}</p><button className="mt-3 inline-flex items-center gap-1 text-zinc-800 hover:underline" onClick={() => void loadDirectory("", true)}><RefreshCw className="size-4" />重试</button></div>}
      {!root?.loading && !root?.error && root?.entries?.length === 0 && <div className="px-3 py-8 text-center text-sm text-zinc-500">此会话尚未创建 Sandbox 或没有可见文件。</div>}
      {!root?.error && renderEntries("")}
    </div>}
  </aside>;
}
