export interface Message {
  id: number
  role: 'user' | 'assistant' | 'tool_call' | 'tool_result' | 'permission_req'
  content?: string
  streaming?: boolean
  // tool_call
  name?: string
  args?: string
  callId?: string
  // tool_result
  result?: string
  collapsed?: boolean
  // permission_req
  requestId?: string
  reason?: string
  responded?: boolean
  approved?: boolean
}

export interface Conversation {
  id: string
  title: string
}

export interface ToolEventData {
  type: 'tool_call' | 'tool_result' | 'permission_request'
  name: string
  args?: string
  result?: string
  call_id?: string
  request_id?: string
  reason?: string
}

export interface SearchResult {
  content?: string
  text?: string
  [key: string]: unknown
}
