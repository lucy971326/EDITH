"use client";

/* eslint-disable @next/next/no-img-element -- local Blob URLs must remain browser-local while upload is in progress. */

import { useEffect, useRef, useState } from "react";

import { uploadChatImage, validateImageFile } from "@/lib/chat/images";
import type { ChatImage } from "@/lib/chat/type";
import type { ModelInfo } from "@/lib/models/type";

type ChatComposerProps = {
  isLoading: boolean;
  isRunning: boolean;
  sessionID: string;
  modelID: string;
  models: ModelInfo[];
  reasoningOptionID: string;
  selectedModel?: ModelInfo;
  onModelChange: (modelID: string) => void;
  onReasoningOptionChange: (reasoningOptionID: string) => void;
  onSend: (message: string, imageIDs: string[]) => void;
};

type SelectedImage = {
  localID: string;
  previewURL: string;
  image?: ChatImage;
  status: "uploading" | "ready" | "failed";
  error?: string;
};

export function ChatComposer({
  isLoading,
  isRunning,
  sessionID,
  modelID,
  models,
  reasoningOptionID,
  selectedModel,
  onModelChange,
  onReasoningOptionChange,
  onSend,
}: ChatComposerProps) {
  const [message, setMessage] = useState("");
  const [images, setImages] = useState<SelectedImage[]>([]);
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

  function send() {
    const content = message.trim();
    if ((!content && readyImageIDs.length === 0) || !modelID || isLoading || isRunning || hasUploadingImage || (hasImageInput && !canUseVision)) {
      return;
    }
    setMessage("");
    setImages((current) => {
      current.forEach((image) => {
        releasePreviewURL(image.previewURL);
      });
      return [];
    });
    onSend(content, readyImageIDs);
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
      className="border-t border-zinc-200 bg-white p-4"
      onSubmit={(event) => {
        event.preventDefault();
        send();
      }}
    >
      <div className="mx-auto max-w-3xl rounded-2xl border border-zinc-300 bg-white p-3 shadow-sm transition-colors focus-within:border-zinc-400">
        {images.length > 0 && <div className="mb-2 flex flex-wrap gap-2 px-1">
          {images.map((image) => <div className="relative h-16 w-16 overflow-hidden rounded-lg border border-zinc-200 bg-zinc-100" key={image.localID}>
            {image.previewURL && <img alt="待发送图片" className="h-full w-full object-cover" src={image.previewURL} />}
            {image.status === "uploading" && <span className="absolute inset-0 flex items-center justify-center bg-white/70 text-zinc-700">↻</span>}
            {image.status === "failed" && <span className="absolute inset-0 flex items-center justify-center bg-red-50 px-1 text-center text-xs text-red-600">{image.error}</span>}
            <button aria-label="移除图片" className="absolute right-1 top-1 flex h-5 w-5 items-center justify-center rounded-full bg-zinc-800/70 text-xs text-white" onClick={() => removeImage(image.localID)} type="button">×</button>
          </div>)}
        </div>}
        <textarea
          className="block min-h-20 w-full resize-none bg-transparent px-1 py-1 text-sm leading-6 outline-none placeholder:text-zinc-400"
          placeholder="输入消息…"
          rows={2}
          value={message}
          disabled={isLoading}
          onChange={(event) => setMessage(event.target.value)}
          onKeyDown={(event) => {
            if (event.key !== "Enter" || event.shiftKey) {
              return;
            }
            event.preventDefault();
            send();
          }}
        />
        <input accept="image/jpeg,image/png,image/webp" className="hidden" multiple onChange={(event) => { void selectImages(event.target.files); event.currentTarget.value = ""; }} ref={fileInput} type="file" />
        <div className="mt-2 flex items-center gap-2 border-t border-zinc-100 pt-2">
          <button aria-label="添加图片" className="flex h-8 w-8 items-center justify-center rounded-lg text-lg text-zinc-500 transition-colors hover:bg-zinc-100 hover:text-zinc-900 disabled:cursor-not-allowed disabled:text-zinc-300" disabled={!canUseVision || isLoading || isRunning} onClick={() => fileInput.current?.click()} title={canUseVision ? "添加图片" : "当前模型不支持图片识别"} type="button">+</button>
          <select className="h-8 max-w-52 rounded-lg bg-transparent px-2 text-sm font-medium text-zinc-700 outline-none hover:bg-zinc-100" disabled={isLoading || isRunning || models.length === 0} value={modelID} onChange={(event) => onModelChange(event.target.value)}>
            {models.length === 0 && <option value="">先在设置配置 API Key</option>}
            {models.map((model) => <option key={model.id} value={model.id}>{model.name}{model.capabilities.vision ? " · 支持识图" : ""}</option>)}
          </select>
          {selectedModel && selectedModel.reasoningOptions.length > 0 && <>
            <span className="text-zinc-300">·</span>
            <select className="h-8 rounded-lg bg-transparent px-2 text-sm text-zinc-600 outline-none hover:bg-zinc-100" disabled={isLoading || isRunning} value={reasoningOptionID} onChange={(event) => onReasoningOptionChange(event.target.value)}>
              <option value="">思考</option>
              {selectedModel.reasoningOptions.map((option) => <option key={option.id} value={option.id}>{option.name}</option>)}
            </select>
          </>}
          <button
            aria-label="发送"
            className="ml-auto flex h-9 w-9 items-center justify-center rounded-xl bg-zinc-900 text-lg text-white transition-colors hover:bg-zinc-700 disabled:cursor-not-allowed disabled:bg-zinc-200 disabled:text-zinc-400"
            disabled={(!message.trim() && readyImageIDs.length === 0) || !modelID || isLoading || isRunning || hasUploadingImage || (hasImageInput && !canUseVision)}
            type="submit"
          >
            {isRunning ? "…" : "↑"}
          </button>
        </div>
      </div>
    </form>
  );
}
