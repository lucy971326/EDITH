// ============================================================================
// Agent API 类型定义
// 后端 Go (main.go) ←→ 前端 TypeScript 的手写契约
// 对应 Go 端的 FrontendEvent / StreamInput / Usage
// ============================================================================

// POST /stream 请求体
export interface StreamRequest {
  user_id: string
  session_id: string
  message: string
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
  text: string
}

/** 推理/思考过程 */
export interface ReasoningEvent {
  type: 'reasoning'
  request_id: string
  thinking: string
}

/** 工具调用发起 */
export interface ToolCallEvent {
  type: 'tool_call'
  request_id: string
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
