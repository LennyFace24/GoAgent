import { ref, reactive, computed, watch, type Ref } from 'vue'
import type { Message, ToolEventData, ApiMessage } from '../../../shared/types'

// ---------- 聊天核心 composable ----------
// 每个会话维护独立的消息缓冲，在途 SSE 流按会话路由，
// 切换会话不会中断后台流——纯前端版"跨会话不中断"。

export interface UseChatOptions {
  conversationId: Ref<string>
  chatMode: Ref<'chat' | 'aiops'>
  scrollToBottom: () => void
  defaultWelcome: string
}

export function useChat(options: UseChatOptions) {
  const { conversationId, chatMode, scrollToBottom, defaultWelcome } = options

  // 每个会话独立的消息缓冲
  const buffers = reactive<Record<string, Message[]>>({})
  // 每个会话的发送状态
  const sendingFlags = reactive<Record<string, boolean>>({})
  // 每个会话的 AbortController
  const abortMap = new Map<string, AbortController>()

  // 当前会话的消息（视图）与发送状态
  const messages = computed<Message[]>(() => buffers[conversationId.value] ?? [])
  const isSending = computed(() => !!sendingFlags[conversationId.value])

  // 切换会话：不中断在途流。仅当缓冲不存在（首次进入该会话）时从后端加载历史。
  watch(conversationId, async (id) => {
    if (!buffers[id]) {
      buffers[id] = []
      await loadHistory(id, defaultWelcome)
    }
  }, { immediate: true })

  function isactive(id: string): boolean {
    return id === conversationId.value
  }

  // 统一的发送函数
  async function send(text: string) {
    if (!text.trim()) return
    const convId = conversationId.value
    if (sendingFlags[convId]) return

    if (!buffers[convId]) buffers[convId] = []
    // 添加用户消息
    buffers[convId].push({
      id: Date.now(),
      role: 'user',
      content: text,
    })
    if (isactive(convId)) scrollToBottom()

    sendingFlags[convId] = true
    const abortCtrl = new AbortController()
    abortMap.set(convId, abortCtrl)
    try {
      const endpoint = chatMode.value === 'chat' ? '/api/chat_stream' : '/api/ai_ops'
      const res = await fetch(endpoint, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify({ message: text, conversation_id: convId }),
        signal: abortCtrl.signal,
      })

      if (!res.ok) {
        buffers[convId].push({
          id: Date.now(),
          role: 'assistant',
          content: `[HTTP ${res.status}] 请求失败`,
        })
        return
      }

      if (!res.body) return
      await readSSEStream(res, abortCtrl, convId)
    } catch (err) {
      console.error('[useChat] 请求错误:', err)
      if (err instanceof Error && err.name === 'AbortError') return
      buffers[convId].push({
        id: Date.now(),
        role: 'assistant',
        content: '请求失败，请重试',
      })
    } finally {
      const buf = buffers[convId]
      const last = buf[buf.length - 1]
      if (last && last.role === 'assistant') {
        last.streaming = false
      }
      if (abortMap.get(convId) === abortCtrl) {
        abortMap.delete(convId)
      }
      sendingFlags[convId] = false
    }
  }

  // 读取 SSE 流（事件路由到 convId 对应的缓冲）
  async function readSSEStream(res: Response, abortCtrl: AbortController, convId: string) {
    const reader = res.body!.getReader()
    const decoder = new TextDecoder()
    let buffer = ''

    while (true) {
      if (abortCtrl.signal.aborted) {
        reader.cancel()
        break
      }

      const { done, value } = await reader.read()
      if (done) break
      buffer += decoder.decode(value, { stream: true })

      // 按双换行分割完整 SSE 事件
      const events = buffer.split('\n\n')
      buffer = events.pop() || ''

      for (const event of events) {
        if (!event.trim()) continue

        let eventType = ''
        let eventData = ''

        for (const line of event.split('\n')) {
          if (line.startsWith('event:')) {
            eventType = line.slice(6).trim()
          } else if (line.startsWith('data:')) {
            eventData = line.slice(5).trim()
          }
        }

        if (!eventData) continue
        // done 事件：结束流
        if (eventData === '"done"' || eventData === 'done') {
          return
        }
        handleSSEEvent(eventType, eventData, convId)
      }
    }
  }

  // 处理 SSE 事件（写入 convId 的缓冲）
  function handleSSEEvent(event: string, payload: string, convId: string) {
    try {
      const data = JSON.parse(payload)
      const buf = buffers[convId]
      if (!buf) return

      // ---- thinking_delta: 流式思维链 ----
      if (event === 'thinking_delta' && data.content) {
        const last = buf[buf.length - 1]
        if (last && last.role === 'thinking' && last.streaming) {
          last.content += data.content
        } else {
          buf.push({
            id: Date.now() + Math.random(),
            role: 'thinking',
            content: data.content,
            streaming: true,
          })
        }
        if (isactive(convId)) scrollToBottom()
        return
      }

      // ---- thinking_done ----
      if (event === 'thinking_done') {
        const last = buf[buf.length - 1]
        if (last && last.role === 'thinking') {
          last.streaming = false
        }
        return
      }

      // ---- tool 事件 ----
      if (event === 'tool') {
        handleToolEvent(data as ToolEventData, convId)
        if (isactive(convId)) scrollToBottom()
        return
      }

      // ---- delta（流式文本）----
      if (event === 'delta' && data.content) {
        appendAssistant(data.content, convId)
        if (isactive(convId)) scrollToBottom()
        return
      }

      // ---- message（非流式文本）----
      if (event === 'message' && data.content) {
        appendAssistant(data.content, convId)
        if (isactive(convId)) scrollToBottom()
        return
      }

      // ---- error ----
      if (event === 'error' || data.error) {
        appendAssistant(`\n\n[错误: ${data.error || '未知错误'}]`, convId)
        return
      }

      // ---- 未知事件，尝试作为 delta ----
      if (data.content) {
        appendAssistant(data.content, convId)
        if (isactive(convId)) scrollToBottom()
      }
    } catch { /* ignore parse errors */ }
  }

  // 追加文本到 assistant 消息
  function appendAssistant(content: string, convId: string) {
    const buf = buffers[convId]
    if (!buf) return
    const last = buf[buf.length - 1]
    if (last && last.role === 'assistant' && last.streaming) {
      last.content += content
    } else {
      buf.push({
        id: Date.now() + Math.random(),
        role: 'assistant',
        content,
        streaming: true,
      })
    }
  }

  // 处理 tool 事件
  function handleToolEvent(ev: ToolEventData, convId: string) {
    const buf = buffers[convId]
    if (!buf) return
    switch (ev.type) {
      case 'tool_call':
        buf.push({
          id: Date.now() + Math.random(),
          role: 'tool_call',
          name: ev.name,
          args: ev.args,
          callId: ev.call_id,
        })
        break
      case 'tool_result':
        buf.push({
          id: Date.now() + Math.random(),
          role: 'tool_result',
          name: ev.name,
          result: ev.result,
          callId: ev.call_id,
          collapsed: true,
        })
        break
      case 'permission_request':
        buf.push({
          id: Date.now() + Math.random(),
          role: 'permission_req',
          requestId: ev.request_id!,
          name: ev.name,
          args: ev.args,
          reason: ev.reason,
          responded: false,
        })
        break
    }
  }

  // 权限响应
  async function respondPermission(msg: Message, approved: boolean, always = false) {
    if (msg.role !== 'permission_req') return
    msg.responded = true
    msg.approved = approved
    try {
      await fetch('/api/permission/response', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify({ id: msg.requestId, conversation_id: conversationId.value, approved, always }),
      })
    } catch { /* ignore */ }
  }

  // 中止当前会话的请求（用户主动停止）
  function abort() {
    const convId = conversationId.value
    const ctrl = abortMap.get(convId)
    if (ctrl) {
      ctrl.abort()
      abortMap.delete(convId)
    }
    // 找到最后一条正在流式的 assistant 消息，保留已回复内容并追加终止标记
    const buf = buffers[convId]
    if (buf) {
      for (let i = buf.length - 1; i >= 0; i--) {
        const m = buf[i]
        if (m.role === 'assistant' && m.streaming) {
          m.streaming = false
          m.content += '\n\n该回答此处被用户终止'
          break
        }
      }
    }
    sendingFlags[convId] = false
  }

  // 加载历史记录到指定会话缓冲
  async function loadHistory(convId: string, defaultMessage: string) {
    try {
      const res = await fetch(`/api/conversation/${convId}`, {
        credentials: 'include',
      })
      if (!res.ok) {
        buffers[convId] = [{ id: Date.now(), role: 'assistant', content: defaultMessage }]
        return
      }
      const data = await res.json()
      const lines: ApiMessage[] = data.messages || []
      const buf: Message[] = []
      for (const line of lines) {
        if (line.role === 'user') {
          buf.push({ id: Date.now() + Math.random(), role: 'user', content: line.content })
        } else if (line.role === 'assistant') {
          if (line.tool_calls) {
            for (const tc of line.tool_calls) {
              buf.push({
                id: Date.now() + Math.random(),
                role: 'tool_call',
                name: tc.function.name,
                args: tc.function.arguments,
                callId: tc.id,
              })
            }
          }
          if (line.content) {
            buf.push({ id: Date.now() + Math.random(), role: 'assistant', content: line.content })
          }
        } else if (line.role === 'tool') {
          buf.push({
            id: Date.now() + Math.random(),
            role: 'tool_result',
            name: line.tool_name || '',
            result: line.content,
            callId: line.tool_call_id || '',
          })
        }
      }
      if (buf.length === 0) {
        buf.push({ id: Date.now(), role: 'assistant', content: defaultMessage })
      }
      buffers[convId] = buf
    } catch {
      buffers[convId] = [{ id: Date.now(), role: 'assistant', content: defaultMessage }]
    }
  }

  return {
    messages,
    isSending,
    send,
    abort,
    respondPermission,
  }
}
