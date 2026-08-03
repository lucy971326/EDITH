"use client";

/* eslint-disable @next/next/no-img-element -- local Blob URLs must remain browser-local while upload is in progress. */

import { ArrowUp, FileUp, ImagePlus, LoaderCircle, Square, X } from "lucide-react";
import { useEffect, useRef, useState } from "react";

import { uploadChatImage, validateImageFile } from "@/lib/chat/images";
import { uploadSandboxFile } from "@/lib/sandbox/files";
import type { ChatImage } from "@/lib/chat/type";
import type { ModelInfo } from "@/lib/models/type";

import { ModelPicker } from "./model-picker";

type ChatComposerProps = {
  isLoading: boolean;
  isRunning: boolean;
  isCancelling: boolean;
  sessionID: string;
  modelID: string;
  models: ModelInfo[];
  reasoningOptionID: string;
  selectedModel?: ModelInfo;
  onCancel: () => void;
  onModelChange: (modelID: string) => void;
  onReasoningOptionChange: (reasoningOptionID: string) => void;
  onSend: (message: string, imageIDs: string[], uploadPaths: string[]) => Promise<boolean>;
};

type SelectedImage = {
  localID: string;
  previewURL: string;
  image?: ChatImage;
  status: "uploading" | "ready" | "failed";
  error?: string;
};
type SelectedFile = { localID: string; file: File; uploadedPath?: string; error?: string };

export function ChatComposer({
  isLoading,
  isRunning,
  isCancelling,
  sessionID,
  modelID,
  models,
  reasoningOptionID,
  selectedModel,
  onCancel,
  onModelChange,
  onReasoningOptionChange,
  onSend,
}: ChatComposerProps) {
  const [message, setMessage] = useState("");
  const [images, setImages] = useState<SelectedImage[]>([]);
  const [files, setFiles] = useState<SelectedFile[]>([]);
  const [isSending, setIsSending] = useState(false);
  const imageInput = useRef<HTMLInputElement>(null);
  const fileInput = useRef<HTMLInputElement>(null);
  const previewURLs = useRef(new Set<string>());
  const canUseVision = selectedModel?.capabilities.vision === true;
  const hasUploadingImage = images.some((image) => image.status === "uploading");
  const readyImageIDs = images.flatMap((image) => image.image ? [image.image.id] : []);
  const hasImageInput = images.length > 0;

  useEffect(() => () => {
    previewURLs.current.forEach((url) => URL.revokeObjectURL(url));
    previewURLs.current.clear();
  }, []);

  function releasePreviewURL(url: string) {
    if (!url) return;
    URL.revokeObjectURL(url);
    previewURLs.current.delete(url);
  }

  async function send() {
    const content = message.trim();
    if ((!content && readyImageIDs.length === 0 && files.length === 0) || !modelID || isLoading || isRunning || isSending || hasUploadingImage || files.some((file) => file.error) || (hasImageInput && !canUseVision)) {
      return;
    }

    // 点击发送后立刻清空界面；若上传或请求失败，再恢复这份草稿。
    const draftImages = images;
    let draftFiles = files;
    setIsSending(true);
    setMessage("");
    setImages([]);
    setFiles([]);
    try {
      const uploadPaths: string[] = [];
      let uploadedFiles = false;
      for (const item of draftFiles) {
        if (item.uploadedPath) {
          uploadPaths.push(item.uploadedPath);
          continue;
        }
        try {
          const uploadedPath = await uploadSandboxFile(sessionID, item.file);
          uploadPaths.push(uploadedPath);
          uploadedFiles = true;
          draftFiles = draftFiles.map((file) => file.localID === item.localID ? { ...file, uploadedPath, error: undefined } : file);
        } catch (error) {
          const errorMessage = error instanceof Error ? error.message : "文件上传失败";
          setMessage(content);
          setImages(draftImages);
          setFiles(draftFiles.map((file) => file.localID === item.localID ? { ...file, error: errorMessage } : file));
          return;
        }
      }
      // 只有实际写入 Sandbox 的文件才通知文件面板刷新；普通消息不影响文件树。
      if (uploadedFiles) {
        window.dispatchEvent(new Event("sandbox-files-updated"));
      }
      const imageIDs = draftImages.flatMap((image) => image.image ? [image.image.id] : []);
      const accepted = await onSend(content, imageIDs, uploadPaths);
      if (!accepted) {
        setMessage(content);
        setImages(draftImages);
        setFiles(draftFiles);
        return;
      }
      draftImages.forEach((image) => releasePreviewURL(image.previewURL));
    } finally {
      setIsSending(false);
    }
  }

  function selectFiles(selected: FileList | null) {
    if (!selected || isLoading || isRunning || isSending) return;
    setFiles((current) => [...current, ...Array.from(selected).map((file) => ({
      localID: crypto.randomUUID(),
      file,
      error: file.size === 0 ? "不能上传空文件" : file.size > 50 * 1024 * 1024 ? "单个文件不能超过 50 MB" : undefined,
    }))]);
  }

  async function selectImages(files: FileList | null) {
    if (!files || !canUseVision || isLoading || isRunning) return;
    for (const file of files) {
      const error = validateImageFile(file);
      if (error) {
        setImages((current) => [...current, {
          localID: crypto.randomUUID(), previewURL: "", status: "failed", error,
        }]);
        continue;
      }

      const localID = crypto.randomUUID();
      const previewURL = URL.createObjectURL(file);
      previewURLs.current.add(previewURL);
      setImages((current) => [...current, { localID, previewURL, status: "uploading" }]);
      try {
        const image = await uploadChatImage(sessionID, file);
        setImages((current) => current.map((item) => item.localID === localID ? { ...item, image, status: "ready" } : item));
      } catch (uploadError) {
        const errorMessage = uploadError instanceof Error ? uploadError.message : "图片上传失败";
        setImages((current) => current.map((item) => item.localID === localID ? { ...item, status: "failed", error: errorMessage } : item));
      }
    }
  }

  function removeImage(localID: string) {
    setImages((current) => {
      const target = current.find((image) => image.localID === localID);
      if (target) releasePreviewURL(target.previewURL);
      return current.filter((image) => image.localID !== localID);
    });
  }

  return (
    <form
      className="bg-canvas/90 px-3 py-2.5 backdrop-blur"
      onSubmit={(event) => {
        event.preventDefault();
        send();
      }}
    >
      <div className="mx-auto w-full max-w-[830px] rounded-[18px] border-[0.9px] border-border bg-surface px-3 py-2 shadow-[0_10px_28px_rgba(20,23,42,0.07)] transition-[border-color,box-shadow] focus-within:border-accent focus-within:shadow-[0_0_0_3px_var(--accent-soft)]">
        {images.length > 0 && <div className="mb-2 flex flex-wrap gap-2 px-1">
          {images.map((image) => <div className="relative h-16 w-16 overflow-hidden rounded-lg border border-border bg-surface-subtle" key={image.localID}>
            {image.previewURL && <img alt="待发送图片" className="h-full w-full object-cover" src={image.previewURL} />}
            {image.status === "uploading" && <span className="absolute inset-0 flex items-center justify-center bg-surface/70 text-ink"><LoaderCircle className="size-4 animate-spin" /></span>}
            {image.status === "failed" && <span className="absolute inset-0 flex items-center justify-center bg-danger-soft px-1 text-center text-xs text-danger">{image.error}</span>}
            <button aria-label="移除图片" className="absolute right-1 top-1 flex h-5 w-5 items-center justify-center rounded-full bg-ink/75 text-xs text-surface" onClick={() => removeImage(image.localID)} type="button">×</button>
          </div>)}
        </div>}
        {files.length > 0 && <div className="mb-2 space-y-1 px-1">
          {files.map((item) => <div className="flex items-center gap-2 rounded-lg bg-surface-subtle px-2 py-1 text-xs text-muted" key={item.localID}><FileUp className="size-3.5" /><span className="truncate">{item.file.name}</span><span className="shrink-0 text-faint">{(item.file.size / 1024 / 1024).toFixed(1)} MB</span>{item.error && <span className="truncate text-danger">{item.error}</span>}<button aria-label="移除文件" className="ml-auto text-muted hover:text-ink" onClick={() => setFiles((current) => current.filter((file) => file.localID !== item.localID))} type="button"><X className="size-3.5" /></button></div>)}
        </div>}
        <textarea
          className="block min-h-[52px] w-full resize-none bg-transparent px-1 py-1 text-sm leading-6 text-ink outline-none placeholder:text-faint"
          placeholder="输入消息…"
          rows={2}
          value={message}
          disabled={isLoading || isSending}
          onChange={(event) => setMessage(event.target.value)}
          onKeyDown={(event) => {
            if (event.key !== "Enter" || event.shiftKey) {
              return;
            }
            event.preventDefault();
            send();
          }}
        />
        <input accept="image/jpeg,image/png,image/webp" className="hidden" multiple onChange={(event) => { void selectImages(event.target.files); event.currentTarget.value = ""; }} ref={imageInput} type="file" />
        <input className="hidden" multiple onChange={(event) => { selectFiles(event.target.files); event.currentTarget.value = ""; }} ref={fileInput} type="file" />
        <div className="mt-1 flex items-center gap-1 border-t border-border pt-1">
          <button aria-label="添加图片" className="ui-icon-button disabled:cursor-not-allowed disabled:text-faint" disabled={!canUseVision || isLoading || isRunning || isSending} onClick={() => imageInput.current?.click()} title={canUseVision ? "添加图片" : "当前模型不支持图片识别"} type="button"><ImagePlus className="size-4" /></button>
          <button aria-label="添加文件" className="ui-icon-button disabled:cursor-not-allowed disabled:text-faint" disabled={isLoading || isRunning || isSending} onClick={() => fileInput.current?.click()} title="添加文件（单个不超过 50MB）" type="button"><FileUp className="size-4" /></button>
          <span className="mx-1 h-4 w-px bg-border" aria-hidden />
          <ModelPicker
              disabled={isLoading || isRunning || isSending}
            modelID={modelID}
            models={models}
            onChange={onModelChange}
          />
          {selectedModel && selectedModel.reasoningOptions.length > 0 && <>
            <span className="text-border-strong">·</span>
            <select className="h-8 rounded-lg bg-transparent px-2 text-sm text-muted outline-none hover:bg-surface-hover" disabled={isLoading || isRunning} value={reasoningOptionID} onChange={(event) => onReasoningOptionChange(event.target.value)}>
              <option value="">思考</option>
              {selectedModel.reasoningOptions.map((option) => <option key={option.id} value={option.id}>{option.name}</option>)}
            </select>
          </>}
          {isRunning ? (
            <button
              aria-label="停止"
              className="ml-auto flex h-8 w-8 items-center justify-center rounded-lg bg-danger text-accent-foreground transition-colors hover:opacity-85 disabled:cursor-wait disabled:opacity-50"
              disabled={isCancelling}
              onClick={(event) => { event.preventDefault(); onCancel(); }}
              type="button"
            >
              {isCancelling ? "…" : <Square className="size-3.5 fill-current" />}
            </button>
          ) : (
            <button
              aria-label="发送"
              className="ml-auto flex h-8 w-8 items-center justify-center rounded-lg bg-accent text-accent-foreground transition-colors hover:bg-accent-hover disabled:cursor-not-allowed disabled:bg-surface-subtle disabled:text-faint"
              disabled={(!message.trim() && readyImageIDs.length === 0 && files.length === 0) || !modelID || isLoading || isSending || hasUploadingImage || files.some((file) => file.error) || (hasImageInput && !canUseVision)}
              type="submit"
            >
              {isSending ? <LoaderCircle className="size-4 animate-spin" /> : <ArrowUp className="size-4" />}
            </button>
          )}
        </div>
      </div>
    </form>
  );
}
