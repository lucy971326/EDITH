import type { AssistantBlock, ToolStatus } from "../../lib/stream";
import type { ChatMessage } from "./types";
import { Icon } from "../../ui/icon";

function toolStatus(status: ToolStatus) {
  return { requested: "准备执行", running: "执行中", completed: "完成", failed: "失败" }[status];
}

function BlockView({ block }: { block: AssistantBlock }) {
  if (block.type === "text") return <p className="assistant-text">{block.content}</p>;
  if (block.type === "reasoning") {
    return <details className="reasoning" open><summary>思考过程</summary><p>{block.content}</p></details>;
  }
  if (block.type === "error") return <p className="error">{block.content}</p>;
  return <details className="tool-card" open={block.status === "running"}><summary><span><Icon name="tool" /> {block.name}</span><span className="tool-status">{toolStatus(block.status)}</span></summary>{block.arguments && <pre>{block.arguments}</pre>}{block.result && <pre>{block.result}</pre>}</details>;
}

export function ChatTimeline({ messages, isRunning }: { messages: ChatMessage[]; isRunning: boolean }) {
  if (messages.length === 0) return <div className="empty-chat">输入一句话，开始当前项目中的 Agent 对话。</div>;
  return <>{messages.map((message) => <article className="message" key={message.id}><div className="message-label"><strong>{message.role === "user" ? "你" : "EDITH"}</strong></div>{message.role === "user" ? <div className="user-message">{message.content}{message.images && message.images.length > 0 && <div className="message-images">{message.images.map((image, index) => <img className="message-image" key={index} src={image.dataUrl} alt={image.name ?? "图片"} />)}</div>}</div> : <div className="assistant-message">{message.blocks.map((block) => <BlockView block={block} key={block.id} />)}{message.blocks.length === 0 && isRunning && <span className="muted">正在思考…</span>}</div>}</article>)}</>;
}
