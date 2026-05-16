// 消息角色类型
export type MessageRole = 'user' | 'assistant' | 'tool_call' | 'tool_result' | 'permission_req'

// 基础消息接口
export interface BaseMessage {
  id: number
  role: MessageRole
  content?: string
  streaming?: boolean
}

// 用户/助手消息
export interface ChatMessage extends BaseMessage {
  role: 'user' | 'assistant'
  content: string
}

// 工具调用消息
export interface ToolCallMessage extends BaseMessage {
  role: 'tool_call'
  name: string
  args?: string
  callId?: string
}

// 工具结果消息
export interface ToolResultMessage extends BaseMessage {
  role: 'tool_result'
  name: string
  result?: string
  callId?: string
  collapsed?: boolean
}

// 权限请求消息
export interface PermissionMessage extends BaseMessage {
  role: 'permission_req'
  requestId: string
  name: string
  args?: string
  reason?: string
  responded?: boolean
  approved?: boolean
}

// 联合消息类型
export type Message = ChatMessage | ToolCallMessage | ToolResultMessage | PermissionMessage

// SSE 工具事件
export interface ToolEventData {
  type: 'tool_call' | 'tool_result' | 'permission_request'
  name: string
  args?: string
  result?: string
  call_id?: string
  request_id?: string
  reason?: string
}
