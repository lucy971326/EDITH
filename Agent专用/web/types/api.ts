// ============================================================================
// Agent API 类型定义
// 后端 Go (main.go) ←→ 前端 TypeScript 的手写契约
// 对应 Go 端的 FrontendEvent / StreamInput / Usage
// ============================================================================

/** 图片输入 */
export interface ImageInput {
  data: string   // base64 编码（不含 data:xxx;base64, 前缀）
  format: string // "png" | "jpeg" | "webp" | "gif"
}

/** 已写入当前会话工作区的用户文件 */
export interface UploadedFile {
  id: string
  name: string
  path: string
  size: number
}

/** 工作区中的一个文件或目录，对应 Go sandbox.FileEntry。 */
export interface FileEntry {
  name: string
  path: string
  type: "file" | "directory"
  size: number
}

/** 工作区目录读取结果，对应 Go WorkspaceListing。 */
export interface WorkspaceListing {
  path: string
  entries: FileEntry[]
}

// POST /stream 请求体
export interface StreamRequest {
  session_id: string
  message: string
  model?: string    // 可选：指定模型名，空则用服务端默认
  images?: ImageInput[] // 可选：图片列表
}

/** /models 返回的模型信息 */
export interface ModelInfo {
  id: string
  label: string
  vision: boolean  // 是否支持图片/视觉输入
}

// SSE 事件联合类型
export type AgentEvent =
  | TextEvent
  | ReasoningEvent
  | ToolCallEvent
  | ToolResultEvent
  | ErrorEvent
  | DoneEvent

// --------------------------------------------------------------------------
// 具体事件
// --------------------------------------------------------------------------

/** 流式文本片段 */
export interface TextEvent {
  type: 'text'
  request_id: string
  /** 这段文本属于哪一次 LLM Response */
  response_id: string
  text: string
}

/** 推理/思考过程 */
export interface ReasoningEvent {
  type: 'reasoning'
  request_id: string
  /** 这段思考属于哪一次 LLM Response */
  response_id: string
  thinking: string
}

/** 工具调用发起 */
export interface ToolCallEvent {
  type: 'tool_call'
  request_id: string
  /** 这次工具调用属于哪一次 LLM Response */
  response_id: string
  id: string
  name: string
  /** JSON 字符串，前端需 JSON.parse */
  arguments: string
}

/** 工具执行结果 */
export interface ToolResultEvent {
  type: 'tool_result'
  request_id: string
  id: string
  name: string
  /** JSON 值（后端保证是合法的 JSON） */
  result: unknown
}

/** 错误 */
export interface ErrorEvent {
  type: 'error'
  request_id: string
  message: string
}

/** Run 结束 */
export interface DoneEvent {
  type: 'done'
  request_id: string
  usage?: TokenUsage
}

/** Token 用量 */
export interface TokenUsage {
  prompt_tokens: number
  completion_tokens: number
  total_tokens: number
}

/** 会话列表 */
export interface SessionInfo {
  id: string
  created_at: string
  updated_at: string
}

/** 会话历史消息（等同于页面内部的 Message 类型，增加了 tool_id） */
export type ChatMessage =
  | { id: string; kind: "user"; text: string }
  | { id: string; response_id: string; kind: "assistant"; text: string; done: boolean }
  | {
      id: string
      response_id: string
      kind: "tool"
      tool_id: string
      name: string
      arguments: string
      result?: unknown
    }
  | { id: string; response_id: string; kind: "reasoning"; text: string }
  | { id: string; kind: "error"; text: string }

/** 会话历史 */
export interface SessionHistory {
  session_id: string
  messages: ChatMessage[]
}
