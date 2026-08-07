import { FormEvent, useRef } from "react";
import type { CommandDefinition } from "../../api/commands";
import type { ModelCatalog } from "../../api/models";
import { Icon } from "../../ui/icon";

export type PendingImage = { id: string; name: string; dataUrl: string };

type ComposerProps = {
  input: string;
  isRunning: boolean;
  isBusy: boolean;
  isStopping: boolean;
  commands: CommandDefinition[];
  modelCatalog: ModelCatalog | null;
  modelID: string;
  thinkingMode: string;
  contextTokens: number | null;
  images: PendingImage[];
  onInput: (value: string) => void;
  onAddImages: (files: File[]) => void;
  onRemoveImage: (id: string) => void;
  onCommandSelect: (syntax: string) => void;
  onModelChange: (modelID: string) => void;
  onThinkingModeChange: (thinkingMode: string) => void;
  onSubmit: (event: FormEvent<HTMLFormElement>) => void;
  onStop: () => void;
};

function formatContextWindow(tokens: number) {
  if (tokens >= 1_000_000) {
    return `${tokens / 1_000_000}M`;
  }
  return `${Math.round(tokens / 1_000)}K`;
}

function formatTokenCount(tokens: number) {
  if (tokens >= 1_000_000) {
    return `${(tokens / 1_000_000).toFixed(1)}M`;
  }
  if (tokens >= 1_000) {
    return `${Math.round(tokens / 1_000)}K`;
  }
  return `${tokens}`;
}

export function Composer({
  input,
  isRunning,
  isBusy,
  isStopping,
  commands,
  modelCatalog,
  modelID,
  thinkingMode,
  contextTokens,
  images,
  onInput,
  onAddImages,
  onRemoveImage,
  onCommandSelect,
  onModelChange,
  onThinkingModeChange,
  onSubmit,
  onStop,
}: ComposerProps) {
  const fileInputRef = useRef<HTMLInputElement>(null);
  const selectedModel = modelCatalog?.models.find((model) => model.id === modelID);
  const thinkingModes = selectedModel?.thinking.modes ?? [];
  const canSubmit = Boolean(input.trim() && modelCatalog && modelID && thinkingMode && !isBusy);
  const commandSuggestions = input.trimStart().startsWith("/") && !isBusy ? commands : [];
  const canUploadImages = Boolean(selectedModel?.vision);

  return (
    <form className="composer" onSubmit={onSubmit}>
      <div className="composer-box">
        <textarea
          value={input}
          onChange={(event) => onInput(event.target.value)}
          disabled={isBusy}
          placeholder="输入消息，使用 / 查看指令与 Skills…"
        />
        {commandSuggestions.length > 0 && (
          <div className="command-suggestions">
            {commandSuggestions.map((command) => (
              <button key={command.name} type="button" onClick={() => onCommandSelect(command.syntax)}>
                <b>{command.syntax}</b>
                <span>{command.description}</span>
              </button>
            ))}
          </div>
        )}
        {images.length > 0 && (
          <div className="composer-images">
            {images.map((image) => (
              <div className="composer-image" key={image.id}>
                <img src={image.dataUrl} alt={image.name} />
                <button aria-label={`移除 ${image.name}`} className="image-remove" onClick={() => onRemoveImage(image.id)} type="button">
                  <Icon name="close" />
                </button>
              </div>
            ))}
          </div>
        )}
        <div className="composer-toolbar">
          <div className="toolbar-group">
            <input
              ref={fileInputRef}
              className="hidden-input"
              type="file"
              accept="image/*"
              multiple
              onChange={(event) => {
                const files = Array.from(event.target.files ?? []);
                if (files.length > 0) {
                  onAddImages(files);
                }
                event.target.value = "";
              }}
            />
            <button
              className="icon-button"
              disabled={isBusy || !canUploadImages}
              onClick={() => fileInputRef.current?.click()}
              title={canUploadImages ? "上传图片" : "当前模型不支持图片"}
              type="button"
            >
              <Icon name="image" />
            </button>
            <span className="control-pill" title="权限设置即将支持">
              <Icon name="shield" />
              <b>normal</b>
              <Icon name="chevron" />
            </span>
          </div>
          <div className="toolbar-group">
            <label className="control-select" title="选择模型">
              <Icon name="spark" />
              <select value={modelID} onChange={(event) => onModelChange(event.target.value)} disabled={isBusy || !modelCatalog}>
                {modelCatalog?.models.map((model) => (
                  <option key={model.id} value={model.id}>{model.id}</option>
                ))}
              </select>
              <Icon name="chevron" />
            </label>
            <label className="control-select" title="选择思考模式">
              <select value={thinkingMode} onChange={(event) => onThinkingModeChange(event.target.value)} disabled={isBusy || !selectedModel}>
                {thinkingModes.map((mode) => <option key={mode} value={mode}>{mode}</option>)}
              </select>
              <Icon name="chevron" />
            </label>
            <span className="control-pill" title="模型上下文窗口">
              Context{" "}
              {contextTokens === null ? (
                selectedModel ? formatContextWindow(selectedModel.contextWindow) : "加载中"
              ) : contextTokens === 0 ? (
                <>暂无用量 / {selectedModel ? formatContextWindow(selectedModel.contextWindow) : "未知"}</>
              ) : (
                <>
                  {formatTokenCount(contextTokens)} / {selectedModel ? formatContextWindow(selectedModel.contextWindow) : "未知"}
                </>
              )}
            </span>
            {isRunning ? (
              <button className="stop-button" disabled={isStopping} onClick={onStop} type="button">
                {isStopping ? "停止中" : "停止"}
              </button>
            ) : (
              <button className="send-button" disabled={!canSubmit} type="submit" title="发送">
                <Icon name="send" />
              </button>
            )}
          </div>
        </div>
      </div>
    </form>
  );
}
