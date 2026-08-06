import { FormEvent } from "react";
import { Icon } from "../../ui/icon";

type ComposerProps = { input: string; isRunning: boolean; isStopping: boolean; onInput: (value: string) => void; onSubmit: (event: FormEvent<HTMLFormElement>) => void; onStop: () => void };

export function Composer({ input, isRunning, isStopping, onInput, onSubmit, onStop }: ComposerProps) {
  return <form className="composer" onSubmit={onSubmit}><div className="composer-box"><textarea value={input} onChange={(event) => onInput(event.target.value)} disabled={isRunning} placeholder="输入消息，使用 / 查看指令与 Skills…" /><div className="composer-toolbar"><div className="toolbar-group"><button className="icon-button" disabled title="文件与图片上传即将支持"><Icon name="attachment" /></button><span className="soon-label">上传即将支持</span><button className="control-pill" type="button" title="权限设置即将支持"><Icon name="shield" /><b>normal</b><Icon name="chevron" /></button></div><div className="toolbar-group"><button className="control-pill" type="button" title="模型选择即将支持"><Icon name="spark" /><b>默认模型</b><Icon name="chevron" /></button><span className="control-pill" title="上下文统计即将支持">Context 即将支持</span>{isRunning ? <button className="stop-button" disabled={isStopping} onClick={onStop} type="button">{isStopping ? "停止中" : "停止"}</button> : <button className="send-button" disabled={!input.trim()} type="submit" title="发送"><Icon name="send" /></button>}</div></div></div></form>;
}
