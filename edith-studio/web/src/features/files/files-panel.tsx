"use client";

import dynamic from "next/dynamic";
import { useEffect, useState } from "react";
import { ListChildren, ReadText } from "../../api/files";
import { Icon } from "../../ui/icon";
import type { FileContent, FileEntry } from "./types";

const MonacoEditor = dynamic(() => import("@monaco-editor/react"), { ssr: false });

type TreeEntry = {
  entry: FileEntry;
  depth: number;
};

function treeEntries(
  entriesByDirectory: Record<string, FileEntry[]>,
  expandedDirectories: Set<string>,
  directory: string,
  depth: number,
): TreeEntry[] {
  const entries = entriesByDirectory[directory] ?? [];
  return entries.flatMap((entry) => {
    const treeEntry = { entry, depth };
    if (entry.kind !== "directory" || !expandedDirectories.has(entry.path)) {
      return [treeEntry];
    }
    return [treeEntry, ...treeEntries(entriesByDirectory, expandedDirectories, entry.path, depth + 1)];
  });
}

export function FilesPanel() {
  const [entriesByDirectory, setEntriesByDirectory] = useState<Record<string, FileEntry[]>>({});
  const [expandedDirectories, setExpandedDirectories] = useState<Set<string>>(() => new Set());
  const [loadingDirectories, setLoadingDirectories] = useState<Set<string>>(() => new Set());
  const [treeError, setTreeError] = useState("");
  const [tabs, setTabs] = useState<FileContent[]>([]);
  const [activePath, setActivePath] = useState<string | null>(null);
  const [fileError, setFileError] = useState("");
  const [loadingFilePath, setLoadingFilePath] = useState<string | null>(null);

  const activeFile = tabs.find((tab) => tab.path === activePath) ?? null;
  const entries = treeEntries(entriesByDirectory, expandedDirectories, "", 0);

  useEffect(() => {
    void loadDirectory("");
  }, []);

  async function loadDirectory(relativeDir: string) {
    setLoadingDirectories((current) => new Set(current).add(relativeDir));
    setTreeError("");
    try {
      const children = await ListChildren(relativeDir);
      setEntriesByDirectory((current) => ({ ...current, [relativeDir]: children }));
    } catch (cause) {
      setTreeError(cause instanceof Error ? cause.message : "无法读取当前项目文件");
    } finally {
      setLoadingDirectories((current) => {
        const next = new Set(current);
        next.delete(relativeDir);
        return next;
      });
    }
  }

  async function toggleDirectory(entry: FileEntry) {
    if (expandedDirectories.has(entry.path)) {
      setExpandedDirectories((current) => {
        const next = new Set(current);
        next.delete(entry.path);
        return next;
      });
      return;
    }
    setExpandedDirectories((current) => new Set(current).add(entry.path));
    if (!Object.hasOwn(entriesByDirectory, entry.path)) {
      await loadDirectory(entry.path);
    }
  }

  async function selectFile(entry: FileEntry) {
    setActivePath(entry.path);
    setFileError("");
    if (tabs.some((tab) => tab.path === entry.path) || loadingFilePath === entry.path) {
      return;
    }
    setLoadingFilePath(entry.path);
    try {
      const content = await ReadText(entry.path);
      setTabs((current) => [...current, content]);
    } catch (cause) {
      setActivePath(null);
      setFileError(cause instanceof Error ? cause.message : "无法读取当前项目文件");
    } finally {
      setLoadingFilePath(null);
    }
  }

  function closeTab(path: string) {
    const nextTabs = tabs.filter((tab) => tab.path !== path);
    setTabs(nextTabs);
    if (path === activePath) {
      setActivePath(nextTabs.at(-1)?.path ?? null);
    }
  }

  return (
    <aside className="inspector">
      <header className="inspector-header">
        <div className="inspector-tabs">
          <button className="inspector-tab active" type="button">代码</button>
          <button className="inspector-tab" type="button" disabled>Diff（即将支持）</button>
        </div>
      </header>
      <div className="files-layout">
        <section className="file-tree">
          <div className="file-tree-title">
            <span>项目文件</span>
            <button aria-label="重新读取项目文件" className="tree-action" onClick={() => void loadDirectory("")} type="button">
              <Icon name="refresh" />
            </button>
          </div>
          {loadingDirectories.has("") && !entriesByDirectory[""] && <p className="file-state">正在读取文件…</p>}
          {treeError && <p className="file-error">无法读取当前项目文件：{treeError}</p>}
          {entries.map(({ entry, depth }) => (
            <button
              className={`tree-row ${entry.path === activePath ? "active" : ""}`}
              key={entry.path}
              onClick={() => entry.kind === "directory" ? void toggleDirectory(entry) : void selectFile(entry)}
              style={{ paddingLeft: `${8 + depth * 15}px` }}
              type="button"
            >
              {entry.kind === "directory" && <Icon className={expandedDirectories.has(entry.path) ? "tree-chevron expanded" : "tree-chevron"} name="chevron" />}
              <Icon name={entry.kind === "directory" ? "folder" : "file"} />
              <span className="tree-name">{entry.name}</span>
              {entry.kind === "directory" && loadingDirectories.has(entry.path) && <span className="tree-loading">读取中</span>}
            </button>
          ))}
        </section>
        <section className="code-panel">
          <div className="file-tabs">
            {tabs.map((tab) => (
              <div className={`file-tab ${tab.path === activePath ? "active" : ""}`} key={tab.path}>
                <button onClick={() => setActivePath(tab.path)} type="button">{fileName(tab.path)}</button>
                <button aria-label={`关闭 ${tab.path}`} className="close-tab" onClick={() => closeTab(tab.path)} type="button">
                  <Icon name="close" />
                </button>
              </div>
            ))}
          </div>
          {loadingFilePath && <div className="code-placeholder">正在读取 {loadingFilePath}…</div>}
          {!loadingFilePath && fileError && <div className="code-placeholder"><p>无法读取当前项目文件：{fileError}</p></div>}
          {!loadingFilePath && !fileError && !activeFile && <div className="code-placeholder"><div><Icon name="file" /><p>从左侧文件树选择一个文件。</p></div></div>}
          {!loadingFilePath && !fileError && activeFile && (
            <div className="editor-area">
              {activeFile.truncated && <p className="truncated-notice">文件超过读取上限，当前只显示前 1 MiB 内容。</p>}
              <MonacoEditor
                language={activeFile.language || "plaintext"}
                options={{ automaticLayout: true, lineNumbers: "on", minimap: { enabled: false }, readOnly: true, scrollBeyondLastLine: false }}
                theme="vs-dark"
                value={activeFile.content}
              />
            </div>
          )}
          <footer className="code-status"><span>只读</span><span>{activeFile?.language || "—"}</span></footer>
        </section>
      </div>
    </aside>
  );
}

function fileName(path: string) {
  return path.split(/[\\/]/).at(-1) ?? path;
}
