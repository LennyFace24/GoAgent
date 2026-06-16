import { ref, type Ref } from 'vue'
import type { Message, ToolEventData } from '../../../shared/types'

// ---------- 聊天核心 composable ----------

export interface UseChatOptions {
  messages: Ref<Message[]>
  conversationId: Ref<string>
  chatMode: Ref<'chat' | 'aiops'>
  scrollToBottom: () => void
}


export function useChat(options: UseChatOptions) {
  const { messages, conversationId, chatMode, scrollToBottom } = options

  const isSending = ref(false)
  let activeAbort: AbortController | null = null

  // 统一的发送函数
  async function send(text: string) {
    if (!text.trim() || isSending.value) return

    // 添加用户消息
    messages.value.push({
      id: Date.now(),
      role: 'user',
      content: text,
    })
    scrollToBottom()

    isSending.value = true
    const abortCtrl = new AbortController()
    activeAbort = abortCtrl
    try {
      // 使用相对路径，通过 Vite 代理访问后端
      const base = ''
      const endpoint = chatMode.value === 'chat' ? '/api/chat_stream' : '/api/ai_ops'
      const res = await fetch(base + endpoint, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify({ message: text, conversation_id: conversationId.value }),
        signal: abortCtrl.signal,
      })




      if (!res.ok) {
        messages.value.push({
          id: Date.now(),
          role: 'assistant',
          content: `[HTTP ${res.status}] 请求失败`,
        })
        return
      }

      if (!res.body) return
      await readSSEStream(res, abortCtrl)
    } catch (err) {
      console.error('[useChat] 请求错误:', err)
      if (err instanceof Error && err.name === 'AbortError') return
      messages.value.push({
        id: Date.now(),
        role: 'assistant',
        content: '请求失败，请重试',
      })
    } finally {
      // 结束流式状态
      const last = messages.value[messages.value.length - 1]
      if (last && last.role === 'assistant') {
        last.streaming = false
      }
      if (activeAbort === abortCtrl) {
        activeAbort = null
      }
      isSending.value = false
    }
  }

  // 读取 SSE 流
  async function readSSEStream(res: Response, abortCtrl: AbortController) {
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
        console.log('[useChat] SSE:', eventType, eventData.slice(0, 80))
        handleSSEEvent(eventType, eventData)

      }
    }
  }


  // 处理 SSE 事件
  function handleSSEEvent(event: string, payload: string) {
    try {
      const data = JSON.parse(payload)


      // ---- thinking_delta: 流式思维链 ----
      if (event === 'thinking_delta' && data.content) {
        const last = messages.value[messages.value.length - 1]
        if (last && last.role === 'thinking' && last.streaming) {
          last.content += data.content
        } else {
          messages.value.push({
            id: Date.now() + Math.random(),
            role: 'thinking',
            content: data.content,
            streaming: true,
          })
        }
        scrollToBottom()
        return
      }

      // ---- thinking_done ----
      if (event === 'thinking_done') {
        const last = messages.value[messages.value.length - 1]
        if (last && last.role === 'thinking') {
          last.streaming = false
        }
        return
      }

      // ---- tool 事件 ----
      if (event === 'tool') {
        handleToolEvent(data as ToolEventData)
        scrollToBottom()
        return
      }

      // ---- delta（流式文本）----
      if (event === 'delta' && data.content) {
        appendAssistant(data.content)
        scrollToBottom()
        return
      }

      // ---- message（非流式文本）----
      if (event === 'message' && data.content) {
        appendAssistant(data.content)
        scrollToBottom()
        return
      }

      // ---- error ----
      if (event === 'error' || data.error) {
        appendAssistant(`\n\n[错误: ${data.error || '未知错误'}]`)
        return
      }

      // ---- 未知事件，尝试作为 delta ----
      if (data.content) {
        appendAssistant(data.content)
        scrollToBottom()
      }
    } catch { /* ignore parse errors */ }
  }

  // 追加文本到 assistant 消息
  function appendAssistant(content: string) {
    const last = messages.value[messages.value.length - 1]
    if (last && last.role === 'assistant' && last.streaming) {
      last.content += content
    } else {
      messages.value.push({
        id: Date.now() + Math.random(),
        role: 'assistant',
        content,
        streaming: true,
      })

    }
  }


  // 处理 tool 事件
  function handleToolEvent(ev: ToolEventData) {
    console.log('[useChat] tool event:', ev.type, ev.name, ev.request_id)
    switch (ev.type) {
      case 'tool_call':
        messages.value.push({
          id: Date.now() + Math.random(),
          role: 'tool_call',
          name: ev.name,
          args: ev.args,
          callId: ev.call_id,
        })
        break
      case 'tool_result':
        messages.value.push({
          id: Date.now() + Math.random(),
          role: 'tool_result',
          name: ev.name,
          result: ev.result,
          callId: ev.call_id,
          collapsed: true,
        })
        break
      case 'permission_request':
        console.warn('[useChat] permission_request:', ev.name, ev.request_id)
        messages.value.push({
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
        body: JSON.stringify({ id: msg.requestId, approved, always }),
      })


    } catch { /* ignore */ }
  }

  // 中止当前请求
  function abort() {
    if (activeAbort) {
      activeAbort.abort()
      activeAbort = null
    }
    isSending.value = false
  }

  // 加载历史记录
  async function loadHistory(defaultMessage: string) {
    messages.value = []

    try {
      const res = await fetch(`/api/conversation/${conversationId.value}`, {
        credentials: 'include',
      })


      if (!res.ok) {
        messages.value.push({ id: Date.now(), role: 'assistant', content: defaultMessage })
        return
      }
      const data = await res.json()
      const lines: Array<{ role: string; content: string; tool_calls?: Array<{ id: string; function: { name: string; arguments: string } }>; tool_call_id?: string; tool_name?: string }> = data.messages || []
      for (const line of lines) {
        if (line.role === 'user') {
          messages.value.push({ id: Date.now() + Math.random(), role: 'user', content: line.content })
        } else if (line.role === 'assistant') {
          if (line.tool_calls) {
            for (const tc of line.tool_calls) {
              messages.value.push({
                id: Date.now() + Math.random(),
                role: 'tool_call',
                name: tc.function.name,
                args: tc.function.arguments,
                callId: tc.id,
              })
            }
          }
          if (line.content) {
            messages.value.push({ id: Date.now() + Math.random(), role: 'assistant', content: line.content })
          }
        } else if (line.role === 'tool') {
          messages.value.push({
            id: Date.now() + Math.random(),
            role: 'tool_result',
            name: line.tool_name || '',
            result: line.content,
            callId: line.tool_call_id || '',
          })
        }
      }
      if (messages.value.length === 0) {
        messages.value.push({ id: Date.now(), role: 'assistant', content: defaultMessage })
      }
    } catch {
      messages.value.push({ id: Date.now(), role: 'assistant', content: defaultMessage })
    }
  }

  return {
    isSending,
    send,
    abort,
    loadHistory,
    respondPermission,
  }
}
